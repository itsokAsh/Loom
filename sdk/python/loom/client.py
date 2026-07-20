import time
import hmac
import hashlib
import json
import uuid
from typing import Dict, Any, NamedTuple
import requests

from .errors import (
    LoomAPIError,
    InvalidSignatureError,
    RateLimitedError,
    WebhookNotFoundError,
    InvalidPayloadError,
    InternalServerError,
)
from .retry import do_with_retry

class TriggerResult(NamedTuple):
    execution_id: str
    status: str

class WebhookClient:
    def __init__(self, base_url: str, path: str, secret: str, timeout: float = 10.0, allow_insecure: bool = False, max_retries: int = 5):
        self.base_url = base_url.rstrip("/")
        self.path = path
        self._secret = secret
        self.timeout = timeout
        self.allow_insecure = allow_insecure
        self.max_retries = max_retries
        self._session = requests.Session()
        
        if not self.allow_insecure and not self.base_url.startswith("https://") and not self.base_url.startswith("http://localhost") and not self.base_url.startswith("http://127.0.0.1"):
            raise ValueError(f"insecure base URL '{self.base_url}' rejected. Use https:// or pass allow_insecure=True")

    def __repr__(self):
        return f"WebhookClient(base_url='{self.base_url}', path='{self.path}', secret='<REDACTED>')"
    
    def __str__(self):
        return self.__repr__()

    def trigger(self, payload: Dict[str, Any]) -> TriggerResult:
        idemp_key = str(uuid.uuid4())
        
        def attempt():
            now_unix = int(time.time())
            envelope = {
                "timestamp": now_unix,
                "payload": payload
            }
            
            # Canonical serialization
            body_bytes = json.dumps(envelope, separators=(',', ':')).encode('utf-8')
            
            signature = hmac.new(
                self._secret.encode('utf-8'),
                body_bytes,
                hashlib.sha256
            ).hexdigest()
            
            headers = {
                "Content-Type": "application/json",
                "X-Signature": signature,
                "Idempotency-Key": idemp_key
            }
            
            req_url = f"{self.base_url}/webhooks/{self.path}"
            
            resp = self._session.post(req_url, data=body_bytes, headers=headers, timeout=self.timeout)
            
            if resp.status_code == 202:
                res_data = resp.json()
                return TriggerResult(
                    execution_id=res_data.get("execution_id", ""),
                    status=res_data.get("status", "")
                )
                
            try:
                err_data = resp.json()
                error_code = err_data.get("error_code", "unknown_error")
                error_msg = err_data.get("error", resp.text)
            except ValueError:
                error_code = "unknown_error"
                error_msg = resp.text
                
            base_err_kwargs = {
                "error_code": error_code,
                "message": error_msg,
                "body": resp.content
            }
            
            if error_code == "invalid_signature":
                raise InvalidSignatureError(**base_err_kwargs)
            elif error_code == "rate_limited":
                retry_after_str = resp.headers.get("Retry-After", "1")
                try:
                    retry_after = float(retry_after_str)
                except ValueError:
                    retry_after = 1.0
                raise RateLimitedError(**base_err_kwargs, retry_after=retry_after)
            elif error_code == "webhook_not_found":
                raise WebhookNotFoundError(**base_err_kwargs)
            elif error_code == "invalid_payload":
                raise InvalidPayloadError(**base_err_kwargs)
            elif error_code == "internal_error" or resp.status_code >= 500:
                raise InternalServerError(**base_err_kwargs)
            else:
                raise LoomAPIError(**base_err_kwargs)
                
        return do_with_retry(attempt, max_retries=self.max_retries)

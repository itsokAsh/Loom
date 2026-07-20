import pytest
import responses
import json
import hmac
import hashlib
from loom import WebhookClient, InvalidPayloadError, RateLimitedError

@responses.activate
def test_signature_correctness():
    secret = "test_secret"
    
    def request_callback(request):
        body = request.body
        signature = hmac.new(
            secret.encode('utf-8'),
            body,
            hashlib.sha256
        ).hexdigest()
        
        assert request.headers.get("X-Signature") == signature, "Signature mismatch"
        return (202, {}, json.dumps({"execution_id": "exec-123", "status": "PENDING"}))
        
    responses.add_callback(
        responses.POST,
        "http://testserver/webhooks/testpath",
        callback=request_callback,
        content_type="application/json",
    )
    
    client = WebhookClient("http://testserver", "testpath", secret, allow_insecure=True)
    res = client.trigger({"foo": "bar"})
    assert res.execution_id == "exec-123"

@responses.activate
def test_retry_eligibility():
    secret = "test_secret"
    
    responses.add(
        responses.POST,
        "http://testserver/webhooks/testpath",
        json={"error_code": "internal_error", "error": "crash"},
        status=500,
    )
    
    responses.add(
        responses.POST,
        "http://testserver/webhooks/testpath",
        json={"execution_id": "exec-123", "status": "PENDING"},
        status=202,
    )
    
    client = WebhookClient("http://testserver", "testpath", secret, allow_insecure=True)
    res = client.trigger({"foo": "bar"})
    assert res.execution_id == "exec-123"
    assert len(responses.calls) == 2

@responses.activate
def test_non_retryable():
    secret = "test_secret"
    
    responses.add(
        responses.POST,
        "http://testserver/webhooks/testpath",
        json={"error_code": "invalid_payload", "error": "bad"},
        status=400,
    )
    
    client = WebhookClient("http://testserver", "testpath", secret, allow_insecure=True)
    with pytest.raises(InvalidPayloadError):
        client.trigger({"foo": "bar"})
    
    assert len(responses.calls) == 1

def test_insecure_rejection():
    with pytest.raises(ValueError, match="insecure base URL"):
        WebhookClient("http://loom.prod", "testpath", "secret")

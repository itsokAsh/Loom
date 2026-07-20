class LoomAPIError(Exception):
    def __init__(self, error_code: str, message: str, body: bytes):
        super().__init__(f"loom API error ({error_code}): {message}")
        self.error_code = error_code
        self.message = message
        self.body = body

class InvalidSignatureError(LoomAPIError):
    pass

class RateLimitedError(LoomAPIError):
    def __init__(self, error_code: str, message: str, body: bytes, retry_after: float):
        super().__init__(error_code, message, body)
        self.retry_after = retry_after

class WebhookNotFoundError(LoomAPIError):
    pass

class InvalidPayloadError(LoomAPIError):
    pass

class InternalServerError(LoomAPIError):
    pass

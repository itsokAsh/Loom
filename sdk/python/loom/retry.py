import time
import random
from typing import Callable, TypeVar
import requests
from requests.exceptions import RequestException
from .errors import (
    LoomAPIError,
    InternalServerError,
    RateLimitedError
)

T = TypeVar('T')

def do_with_retry(
    attempt_func: Callable[[], T],
    max_retries: int = 5,
    base_delay: float = 0.5,
    max_delay: float = 5.0
) -> T:
    last_err = None
    
    for attempt in range(max_retries):
        try:
            return attempt_func()
        except RateLimitedError as e:
            last_err = e
            delay = e.retry_after
        except (InternalServerError, RequestException) as e:
            last_err = e
            backoff = min(base_delay * (2 ** attempt), max_delay)
            jitter = random.uniform(0, 1.0)
            delay = backoff + jitter
        except LoomAPIError as e:
            # All other API errors (e.g., 400, 401, 404) are non-retryable
            raise e
            
        time.sleep(delay)
        
    raise last_err

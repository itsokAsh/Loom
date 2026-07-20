import sys
import os

# Add parent directory to path so we can import loom
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from loom import WebhookClient, RateLimitedError, InvalidSignatureError

def main():
    if len(sys.argv) < 3:
        print("Usage: python main.py <webhook_path> <webhook_secret>")
        sys.exit(1)
        
    path = sys.argv[1]
    secret = sys.argv[2]
    
    try:
        client = WebhookClient(
            base_url="http://localhost:8081/v1",
            path=path,
            secret=secret,
            allow_insecure=True,
            max_retries=3
        )
    except ValueError as e:
        print(f"Failed to initialize client: {e}")
        sys.exit(1)
        
    payload = {
        "event": "user_signup",
        "data": {
            "email": "test_py@example.com"
        }
    }
    
    print("Triggering webhook...")
    try:
        res = client.trigger(payload)
        print(f"Successfully triggered workflow!\nExecution ID: {res.execution_id}\nStatus: {res.status}")
    except RateLimitedError as e:
        print(f"Rate limited! Try again after {e.retry_after}s")
    except InvalidSignatureError:
        print("Invalid signature - did you provide the right secret?")
    except Exception as e:
        print(f"Failed to trigger webhook: {e}")

if __name__ == "__main__":
    main()

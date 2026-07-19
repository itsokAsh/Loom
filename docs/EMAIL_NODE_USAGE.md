# Email Node Usage Guide

## Overview

The Email node sends emails via SendGrid with built-in security protections including:
- SSRF protection (blocks requests to private IPs and cloud metadata endpoints)
- DNS rebinding prevention
- Rate limiting (100 emails max per workflow execution)
- Recipient limits (10 max per email)
- Header injection prevention

## Configuration

### Environment Setup

1. Get a SendGrid API key from https://app.sendgrid.com/settings/api_keys
2. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```
3. Add your SendGrid API key to `.env`:
   ```
   SENDGRID_API_KEY=SG.your_actual_key_here
   ```

### Email Node Config

```json
{
  "id": "send_email",
  "type": "EMAIL",
  "config": {
    "to": "user@example.com",
    "subject": "Welcome!",
    "body": "Welcome to our platform!",
    "from": "noreply@yourdomain.com",
    "cc": ["manager@example.com"],
    "bcc": ["archive@example.com"]
  }
}
```

### Config Fields

| Field | Required | Description | Limits |
|-------|----------|-------------|--------|
| `to` | Yes | Recipient email address | Valid email format |
| `subject` | Yes | Email subject line | Max 200 characters, no CRLF |
| `body` | Yes | Email body (plain text) | Max 50,000 characters |
| `from` | No | Sender email address | Valid email format, defaults to noreply@loom.com |
| `cc` | No | CC recipients | Array of valid emails |
| `bcc` | No | BCC recipients | Array of valid emails |

### Rate Limits

- **Per execution:** Max 100 emails (enforced in orchestration engine)
- **Per email:** Max 10 total recipients (to + cc + bcc combined)

## Template Variables

Use `{{trigger.*}}` or `{{outputs.*}}` to interpolate data:

```json
{
  "to": "{{trigger.email}}",
  "subject": "Welcome {{trigger.name}}!",
  "body": "Hi {{trigger.name}},\n\nThanks for signing up!"
}
```

## Example Workflows

### User Onboarding

```json
{
  "name": "User Onboarding",
  "nodes": [
    {
      "id": "send_welcome",
      "type": "EMAIL",
      "config": {
        "to": "{{trigger.email}}",
        "subject": "Welcome to Our Platform!",
        "body": "Hi {{trigger.name}},\n\nWelcome aboard!"
      }
    }
  ],
  "edges": []
}
```

### Password Reset

```json
{
  "name": "Password Reset",
  "nodes": [
    {
      "id": "send_reset_link",
      "type": "EMAIL",
      "config": {
        "to": "{{trigger.email}}",
        "subject": "Password Reset Request",
        "body": "Click here to reset: {{trigger.reset_url}}"
      }
    }
  ],
  "edges": []
}
```

## Security Features

### SSRF Protection

The email node uses a hardened HTTP engine that blocks:
- Private IP ranges (10.x, 172.16-31.x, 192.168.x)
- Localhost (127.0.0.1, ::1)
- Link-local addresses (169.254.x.x)
- Cloud metadata endpoints (169.254.169.254)

### DNS Rebinding Prevention

The HTTP engine re-validates DNS on every connection, preventing attacks where:
1. First DNS lookup returns a safe IP
2. Attacker changes DNS to point to private IP
3. Connection attempt uses the new (blocked) IP and fails

### Fixed Destination

Email nodes always send to SendGrid's API (`https://api.sendgrid.com/v3/mail/send`). The destination cannot be changed via workflow configuration, preventing SSRF attacks.

## Error Handling

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `secret not found: SENDGRID_API_KEY` | API key not set | Add SENDGRID_API_KEY to environment |
| `invalid recipient email` | Invalid email format | Check email format |
| `subject exceeds max length` | Subject > 200 chars | Shorten subject |
| `too many recipients` | More than 10 recipients | Reduce recipients or split into multiple emails |
| `email limit exceeded` | More than 100 emails in execution | Execution hit rate limit |
| `SendGrid API error: status 401` | Invalid API key | Check API key is correct |
| `SendGrid API error: status 403` | API key lacks permissions | Grant "Mail Send" permission to API key |

### Retry Behavior

- Failed email sends are automatically retried (up to 3 attempts)
- Retries are handled by the orchestration engine
- Each retry attempt is tracked in the database

## Monitoring

### Success Response

```json
{
  "status": "sent",
  "sendgrid_message_id": "abc123...",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Logs

The worker logs include:
- Dispatch ID (for deduplication tracking)
- Email validation failures
- API errors (with redacted API keys)
- SSRF block attempts

## Testing

### Test with httpbin

Create a test workflow to verify the HTTP engine works:

```json
{
  "id": "test_http",
  "type": "HTTP",
  "config": {
    "method": "POST",
    "url": "https://httpbin.org/post",
    "body": {"test": "data"}
  }
}
```

### Test Email Send

1. Set up a test SendGrid API key
2. Create a workflow with an email node
3. Trigger the workflow via webhook
4. Check SendGrid Activity dashboard for delivery status

## Production Checklist

- [ ] SendGrid API key configured
- [ ] API key has "Mail Send" permission
- [ ] From address verified in SendGrid
- [ ] Rate limits appropriate for use case
- [ ] Error monitoring configured
- [ ] Email delivery monitoring set up

## Troubleshooting

### Email not received

1. Check SendGrid Activity dashboard
2. Verify recipient email is valid
3. Check spam/junk folder
4. Verify from address is verified in SendGrid
5. Check workflow execution logs for errors

### Rate limit exceeded

If you hit the 100 email limit:
1. Check workflow logic (are you sending more emails than expected?)
2. Consider splitting into multiple workflow executions
3. Contact support to adjust limits if needed

### SSRF blocks

If legitimate requests are being blocked:
1. Check destination hostname
2. Verify hostname resolves to public IP
3. Check logs for specific blocked IP
4. Contact support if blocking is incorrect

## Support

For issues with:
- **SendGrid**: Check SendGrid documentation and support
- **Email node**: Check workflow execution logs and this documentation
- **SSRF blocks**: Contact Loom support with specific details

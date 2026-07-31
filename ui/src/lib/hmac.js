export async function signWebhookPayload(secret, payload, idempotencyKey) {
  const envelope = {
    timestamp: Math.floor(Date.now() / 1000),
    payload,
  };
  const body = JSON.stringify(envelope);
  const enc = new TextEncoder();
  const key = await crypto.subtle.importKey(
    'raw',
    enc.encode(secret),
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign']
  );
  const sig = await crypto.subtle.sign('HMAC', key, enc.encode(body));
  const hex = Array.from(new Uint8Array(sig))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');

  const headers = {
    'Content-Type': 'application/json',
    'X-Signature': hex,
  };
  if (idempotencyKey) headers['Idempotency-Key'] = idempotencyKey;

  return { body, headers };
}

export async function triggerWebhook(webhookPath, secret, payload, idempotencyKey) {
  const base = import.meta.env.VITE_WEBHOOK_BASE || '/v1/webhooks';
  const { body, headers } = await signWebhookPayload(secret, payload, idempotencyKey);
  const res = await fetch(`${base}/${webhookPath}`, { method: 'POST', headers, body });
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = text;
  }
  if (!res.ok) {
    throw new Error(typeof data === 'string' ? data : JSON.stringify(data));
  }
  return data;
}

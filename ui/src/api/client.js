const API_URL = import.meta.env.VITE_API_URL || '/v1';
const ADMIN_KEY = import.meta.env.VITE_ADMIN_API_KEY || 'dev-admin-key';

export class ApiError extends Error {
  constructor(message, status, body) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

function friendlyApiError(text, status) {
  const raw = (text || '').trim();
  if (raw.startsWith('<')) {
    if (status === 502 || status === 503) {
      return 'Loom API is not reachable (backend may still be starting). Run docker compose up -d, wait ~30s, then Retry.';
    }
    return `Server error (${status}). Check that trigger-api is running on port 8080.`;
  }
  if (raw.length > 200) {
    return raw.slice(0, 200) + '…';
  }
  return raw || `Request failed (${status})`;
}

export async function apiRequest(path, options = {}) {
  const url = `${API_URL}${path}`;
  const headers = {
    'Content-Type': 'application/json',
    'X-Admin-API-Key': ADMIN_KEY,
    Authorization: `Bearer ${ADMIN_KEY}`,
    ...(options.headers || {}),
  };
  const res = await fetch(url, { ...options, headers });
  const text = await res.text();
  let data = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }
  if (!res.ok) {
    const msg =
      typeof data === 'object' && data?.error
        ? data.error
        : friendlyApiError(typeof data === 'string' ? data : text, res.status);
    throw new ApiError(msg || 'Request failed', res.status, data);
  }
  return data;
}

export function getApiBase() {
  return API_URL;
}

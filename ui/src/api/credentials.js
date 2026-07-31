import { apiRequest } from './client';

export function listCredentials() {
  return apiRequest('/credentials');
}

export function createCredential({ name, type = 'sendgrid', apiKey }) {
  return apiRequest('/credentials', {
    method: 'POST',
    body: JSON.stringify({ name, type, apiKey }),
  });
}

export function deleteCredential(id) {
  return apiRequest(`/credentials/${id}`, { method: 'DELETE' });
}

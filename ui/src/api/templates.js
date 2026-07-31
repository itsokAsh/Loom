import { apiRequest } from './client';

export function listTemplates(category = '') {
  const q = category ? `?category=${encodeURIComponent(category)}` : '';
  return apiRequest(`/templates${q}`);
}

export function createFromTemplate(templateId, config) {
  return apiRequest(`/templates/${templateId}/create`, {
    method: 'POST',
    body: JSON.stringify({ config }),
  });
}

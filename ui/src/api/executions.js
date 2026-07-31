import { apiRequest } from './client';

export function getExecution(id) {
  return apiRequest(`/executions/${id}`);
}

export function getExecutionNodes(id) {
  return apiRequest(`/executions/${id}/nodes`);
}

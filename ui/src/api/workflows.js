import { apiRequest } from './client';

export function listWorkflows() {
  return apiRequest('/workflows');
}

export function getWorkflow(id) {
  return apiRequest(`/workflows/${id}`);
}

export function createWorkflow(name, dag) {
  return apiRequest('/workflows', {
    method: 'POST',
    body: JSON.stringify({ name, dag }),
  });
}

export function updateWorkflow(id, name) {
  return apiRequest(`/workflows/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ name }),
  });
}

export function deleteWorkflow(id) {
  return apiRequest(`/workflows/${id}`, { method: 'DELETE' });
}

export function saveWorkflowVersion(id, version, dag) {
  return apiRequest(`/workflows/${id}/versions`, {
    method: 'POST',
    body: JSON.stringify({ version, dag }),
  });
}

export function validateDAG(dag) {
  return apiRequest('/workflows/validate', {
    method: 'POST',
    body: JSON.stringify({ dag }),
  });
}

export function listExecutions(workflowId) {
  return apiRequest(`/workflows/${workflowId}/executions`);
}

export function listWebhooks(workflowId) {
  return apiRequest(`/workflows/${workflowId}/webhooks`);
}

export function createWebhook(workflowId) {
  return apiRequest(`/workflows/${workflowId}/webhooks`, { method: 'POST' });
}

export function listSchedules(workflowId) {
  return apiRequest(`/workflows/${workflowId}/schedules`);
}

export function createSchedule(workflowId, cronExpression) {
  return apiRequest(`/workflows/${workflowId}/schedules`, {
    method: 'POST',
    body: JSON.stringify({ cronExpression }),
  });
}

export function deleteSchedule(workflowId, scheduleId) {
  return apiRequest(`/workflows/${workflowId}/schedules/${scheduleId}`, {
    method: 'DELETE',
  });
}

export function executeWorkflow(workflowId, triggerData, idempotencyKey) {
  const body = { triggerData: triggerData || {} };
  if (idempotencyKey) body.idempotencyKey = idempotencyKey;
  return apiRequest(`/workflows/${workflowId}/execute`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

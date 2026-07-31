import { isStartType } from './nodeCatalog';

export function validateFlowClient(nodes, edges) {
  const errors = [];
  const start = nodes.filter((n) => isStartType(n.data?.nodeType));
  if (start.length === 0) {
    errors.push('Add a Start node (Manual Trigger, Webhook, or Schedule).');
  }
  if (start.length > 1) {
    errors.push('Only one Start node is allowed.');
  }
  const workerNodes = nodes.filter((n) => !isStartType(n.data?.nodeType));
  if (workerNodes.length === 0) {
    errors.push('Add what should run after the trigger: HTTP Request, Send Email, or Transform (not a Webhook).');
  }
  const ids = new Set();
  for (const n of workerNodes) {
    if (ids.has(n.id)) errors.push(`Duplicate node id: ${n.id}`);
    ids.add(n.id);
    const cfg = n.data?.config || {};
    if (n.data.nodeType === 'EMAIL' && containsTrigger(cfg.from)) {
      errors.push(`Email "From" cannot use trigger data — use a fixed address.`);
    }
    if (n.data.nodeType === 'HTTP' && containsTrigger(cfg.url)) {
      errors.push(`HTTP URL cannot use trigger data.`);
    }
  }
  return errors;
}

function containsTrigger(val) {
  if (typeof val !== 'string') return false;
  return val.includes('{{trigger');
}

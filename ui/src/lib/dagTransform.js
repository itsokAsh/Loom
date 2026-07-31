import { NODE_CATALOG, isStartType } from './nodeCatalog';

const TYPE_COLORS = {
  MANUAL: '#f59e0b',
  WEBHOOK: '#f59e0b',
  SCHEDULE: '#f59e0b',
  TRIGGER: '#f59e0b',
  EMAIL: '#10b981',
  HTTP: '#3b82f6',
  TRANSFORM: '#8b5cf6',
};

export function defaultConfig(type) {
  switch (type) {
    case 'EMAIL':
      return {
        to: '{{trigger.email}}',
        from: 'noreply@yourdomain.com',
        subject: 'Hello {{trigger.name}}',
        body: 'Hi {{trigger.name}},\n\nWelcome!',
        credentialId: '',
      };
    case 'HTTP':
      return { method: 'GET', url: 'https://jsonplaceholder.typicode.com/todos/1', headers: {}, body: null };
    case 'TRANSFORM':
      return { mapping: { email: '{{trigger.email}}', name: '{{trigger.name}}' } };
    case 'SCHEDULE':
      return { cronExpression: '0 9 * * *' };
    case 'MANUAL':
    case 'WEBHOOK':
      return {};
    default:
      return {};
  }
}

export function newNodeId(type) {
  const base = type.toLowerCase().replace(/[^a-z]/g, '');
  return `${base}_${Math.random().toString(36).slice(2, 8)}`;
}

export function friendlyLabel(type, existingLabel) {
  if (existingLabel && !/^[a-z]+_[a-z0-9]+$/i.test(existingLabel)) {
    return existingLabel;
  }
  return NODE_CATALOG[type]?.name || type;
}

/** Load API DAG (may include ui canvas) into React Flow */
export function loomDagToFlow(dag, nodeStatuses = {}) {
  if (dag?.ui?.nodes?.length) {
    const nodes = dag.ui.nodes.map((n) => {
      const nodeType = n.data?.nodeType || n.type?.toUpperCase?.() || n.nodeType;
      const isStart = isStartType(nodeType);
      return {
        id: n.id,
        type: isStart ? 'trigger' : 'loom',
        position: n.position || { x: 200, y: 80 },
        data: {
          label: n.data?.label || friendlyLabel(nodeType),
          nodeType,
          config: n.data?.config || {},
          status: nodeStatuses[n.id] || 'idle',
        },
      };
    });
    const edges = (dag.ui.edges || []).map((e, i) => ({
      id: e.id || `e-${e.source}-${e.target}-${i}`,
      source: e.source,
      target: e.target,
      label: e.data?.condition || e.label || '',
      data: { condition: e.data?.condition || e.condition || '' },
    }));
    return { nodes, edges };
  }

  if (!dag?.nodes) {
    return { nodes: [], edges: [] };
  }

  const nodes = dag.nodes.map((n, i) => {
    const col = i % 3;
    const row = Math.floor(i / 3);
    const isStart = isStartType(n.type);
    return {
      id: n.id,
      type: isStart ? 'trigger' : 'loom',
      position: n.position || { x: 80 + col * 240, y: 80 + row * 120 },
      data: {
        label: n.label || friendlyLabel(n.type),
        nodeType: n.type,
        config: n.config || {},
        status: nodeStatuses[n.id] || 'idle',
      },
    };
  });

  const edges = (dag.edges || []).map((e, i) => ({
    id: `e-${e.source}-${e.target}-${i}`,
    source: e.source,
    target: e.target,
    label: e.condition || '',
    data: { condition: e.condition || '' },
  }));

  return { nodes, edges };
}

/**
 * Save format: worker nodes/edges + full ui canvas for reload.
 * Start nodes (MANUAL/WEBHOOK/SCHEDULE) live only in ui.
 */
export function flowToLoomDag(nodes, edges) {
  const workerNodes = nodes.filter((n) => !isStartType(n.data?.nodeType));
  const workerIds = new Set(workerNodes.map((n) => n.id));

  const dag = {
    nodes: workerNodes.map((n) => ({
      id: n.id,
      type: n.data.nodeType,
      config: sanitizeConfig(n.data.nodeType, n.data.config || {}),
      label: n.data.label,
    })),
    edges: edges
      .filter((e) => workerIds.has(e.source) && workerIds.has(e.target))
      .map((e) => ({
        source: e.source,
        target: e.target,
        ...(e.data?.condition ? { condition: e.data.condition } : {}),
      })),
    ui: {
      nodes: nodes.map((n) => ({
        id: n.id,
        position: n.position,
        data: {
          label: n.data.label,
          nodeType: n.data.nodeType,
          config: sanitizeConfig(n.data.nodeType, n.data.config || {}),
        },
      })),
      edges: edges.map((e) => ({
        id: e.id,
        source: e.source,
        target: e.target,
        data: { condition: e.data?.condition || '' },
      })),
    },
  };
  return dag;
}

function sanitizeConfig(type, cfg) {
  const copy = { ...cfg };
  delete copy.apiKey;
  delete copy._sendgrid_api_key;
  if (type === 'EMAIL' && copy.credentialId === undefined) {
    copy.credentialId = '';
  }
  return copy;
}

export function getStartNode(nodes) {
  return nodes.find((n) => isStartType(n.data?.nodeType));
}

export { TYPE_COLORS };

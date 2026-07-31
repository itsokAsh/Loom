import { Handle, Position } from 'reactflow';
import { Check, X, Loader2, Minus } from 'lucide-react';
import { TYPE_COLORS } from '../../lib/dagTransform';
import { NODE_CATALOG, isStartType } from '../../lib/nodeCatalog';

const STATUS_RING = {
  idle: 'transparent',
  running: '#3b82f6',
  SUCCESS: '#10b981',
  success: '#10b981',
  ERROR: '#ef4444',
  error: '#ef4444',
  FAILED: '#ef4444',
  QUEUED: '#f59e0b',
  RUNNING: '#3b82f6',
  SKIPPED: '#6b7280',
};

function StatusBadge({ status }) {
  const s = (status || 'idle').toUpperCase();
  if (!status || status === 'idle') return null;

  if (s === 'RUNNING' || s === 'QUEUED') {
    return (
      <span className="node-status-badge badge-running" title={s}>
        <Loader2 size={12} className="spin" />
      </span>
    );
  }
  if (s === 'SUCCESS') {
    return (
      <span className="node-status-badge badge-success" title="SUCCESS">
        <Check size={12} />
      </span>
    );
  }
  if (s === 'ERROR' || s === 'FAILED') {
    return (
      <span className="node-status-badge badge-error" title={s}>
        <X size={12} />
      </span>
    );
  }
  if (s === 'SKIPPED') {
    return (
      <span className="node-status-badge badge-skipped" title="SKIPPED">
        <Minus size={12} />
      </span>
    );
  }
  return null;
}

export function LoomNode({ data, selected }) {
  const type = data.nodeType || 'HTTP';
  const catalog = NODE_CATALOG[type];
  const Icon = catalog?.Icon;
  const color = catalog?.color || TYPE_COLORS[type] || '#6b7280';
  const ring = STATUS_RING[data.status] || STATUS_RING.idle;

  return (
    <div
      className={`loom-node ${selected ? 'selected' : ''}`}
      style={{ borderColor: ring !== 'transparent' ? ring : undefined }}
    >
      <StatusBadge status={data.status} />
      <Handle type="target" position={Position.Top} className="loom-handle" />
      <div className="loom-node-icon" style={{ color }}>
        {Icon ? <Icon size={18} /> : null}
      </div>
      <div className="loom-node-text">
        <div className="loom-node-title">{data.label || catalog?.name || type}</div>
        <div className="loom-node-sub">{catalog?.subtitle || type}</div>
      </div>
      <Handle type="source" position={Position.Bottom} className="loom-handle" />
    </div>
  );
}

export function TriggerNode({ data, selected }) {
  const type = data.nodeType || 'MANUAL';
  const catalog = NODE_CATALOG[type] || NODE_CATALOG.MANUAL;
  const Icon = catalog.Icon;
  const color = catalog.color;
  const ring = STATUS_RING[data.status] || STATUS_RING.idle;

  return (
    <div
      className={`loom-node trigger-node ${selected ? 'selected' : ''}`}
      style={{ borderColor: ring !== 'transparent' ? ring : undefined }}
    >
      <StatusBadge status={data.status} />
      <div className="loom-node-icon" style={{ color }}>
        <Icon size={18} />
      </div>
      <div className="loom-node-text">
        <div className="loom-node-title">{data.label || catalog.name}</div>
        <div className="loom-node-sub">{catalog.subtitle}</div>
      </div>
      <Handle type="source" position={Position.Bottom} className="loom-handle" />
    </div>
  );
}

export { isStartType };

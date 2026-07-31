import { CheckCircle2, Circle } from 'lucide-react';
import { getReadinessChecks } from '../lib/workflowReadiness';

export default function WorkflowReadiness({ nodes, activeId, dirty, webhooks, schedules, hasRun, onAction }) {
  const checks = getReadinessChecks({ nodes, activeId, dirty, webhooks, schedules, hasRun });
  const done = checks.filter((c) => c.ok).length;

  return (
    <div className="readiness-panel">
      <div className="readiness-header">
        <span className="readiness-title">Ready?</span>
        <span className="readiness-count">{done}/{checks.length}</span>
      </div>
      <ul className="readiness-list">
        {checks.map((c) => (
          <li key={c.id} className={c.ok ? 'ok' : 'pending'}>
            {c.ok ? (
              <CheckCircle2 size={14} className="readiness-icon ok" />
            ) : (
              <Circle size={14} className="readiness-icon" />
            )}
            <div className="readiness-item-body">
              <button
                type="button"
                className="readiness-label"
                onClick={() => onAction(c.action)}
                disabled={c.ok}
              >
                {c.label}
              </button>
              {!c.ok && <p className="readiness-fix">{c.fix}</p>}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

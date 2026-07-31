import { useEffect, useState } from 'react';
import { CheckCircle2 } from 'lucide-react';
import { NODE_CATALOG, isStartType } from '../lib/nodeCatalog';
import * as credApi from '../api/credentials';

function Field({ label, hint, children }) {
  return (
    <div className="field">
      <label>{label}</label>
      {children}
      {hint && <p className="field-hint">{hint}</p>}
    </div>
  );
}

function parseCredId(c) {
  if (!c) return '';
  const id = c.id ?? c.ID;
  if (typeof id === 'string') return id;
  if (id?.String) return id.String;
  try {
    if (id?.Bytes) {
      const b = Object.values(id.Bytes);
      if (b.length === 16) {
        const hex = b.map((x) => Number(x).toString(16).padStart(2, '0')).join('');
        return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
      }
    }
  } catch {
    /* ignore */
  }
  return String(id);
}

function parseScheduleId(s) {
  const id = s?.id ?? s?.ID;
  return typeof id === 'string' ? id : String(id);
}

function scheduleCron(s) {
  return s.cronExpression || s.CronExpression || s.cron_expression || '';
}

function scheduleNextRun(s) {
  return s.nextRunAt || s.NextRunAt || s.next_run_at;
}

export default function NodeConfigForm({
  node,
  onChange,
  onRename,
  webhooks = [],
  onCreateWebhook,
  webhookUrl,
  schedules = [],
  onCreateSchedule,
  onStopSchedule,
  formatCronTime,
  canCreateWebhook,
  allNodes = [],
  allEdges = [],
  lastRun,
  toast,
}) {
  const type = node.data.nodeType;
  const cfg = node.data.config || {};
  const [credentials, setCredentials] = useState([]);
  const [showCredModal, setShowCredModal] = useState(false);
  const [credName, setCredName] = useState('SendGrid');
  const [credKey, setCredKey] = useState('');
  const [revealSecret, setRevealSecret] = useState(false);
  const [credError, setCredError] = useState('');

  useEffect(() => {
    if (type === 'EMAIL') {
      credApi.listCredentials().then((list) => setCredentials(list || [])).catch(() => setCredentials([]));
    }
  }, [type, showCredModal]);

  const mapFromPreviousHttp = () => {
    const incoming = allEdges.filter((e) => e.target === node.id).map((e) => e.source);
    const http = allNodes.find((n) => incoming.includes(n.id) && n.data?.nodeType === 'HTTP');
    if (!http) return false;
    onChange('mapping', {
      status: `{{ outputs.${http.id}.status }}`,
      body: `{{ outputs.${http.id}.body }}`,
    });
    return true;
  };

  if (isStartType(type)) {
    return (
      <div className="node-config-intro">
        <Field label="Display name">
          <input
            value={node.data.label || ''}
            onChange={(e) => onRename(e.target.value)}
          />
        </Field>
        <p className="config-desc">{NODE_CATALOG[type]?.description}</p>

        {type === 'MANUAL' && (
          <p className="field-hint">
            Open the <strong>Test</strong> tab → edit sample JSON → <strong>Run</strong>. No webhook needed.
            You still need a next step (HTTP, Email, or Transform).
          </p>
        )}

        {type === 'WEBHOOK' && (
          <div className="webhook-params">
            {!canCreateWebhook && (
              <p className="field-hint">Save the workflow first, then create a URL here.</p>
            )}
            <button type="button" className="btn-sm" onClick={onCreateWebhook} disabled={!canCreateWebhook || webhooks.length > 0}>
              {webhooks.length ? 'URL created' : 'Create webhook URL'}
            </button>
            {webhooks.map((wh) => {
              const url = webhookUrl(wh);
              const secret = wh.secret || wh.Secret || '';
              return (
                <div key={wh.id || wh.Path} className="webhook-card">
                  <Field label="Production URL" hint="Share this with apps that should start the workflow.">
                    <code className="mono-block">{url}</code>
                    <button type="button" className="btn-xs" onClick={() => navigator.clipboard.writeText(url)}>Copy URL</button>
                  </Field>
                  <Field label="Secret" hint="Used to sign requests. Hidden by default.">
                    <code className="mono-block">{revealSecret ? secret : '••••••••••••••••'}</code>
                    <div className="btn-row">
                      <button type="button" className="btn-xs" onClick={() => setRevealSecret((v) => !v)}>
                        {revealSecret ? 'Hide' : 'Reveal'}
                      </button>
                      <button type="button" className="btn-xs" onClick={() => navigator.clipboard.writeText(secret)}>Copy secret</button>
                    </div>
                  </Field>
                </div>
              );
            })}
          </div>
        )}

        {type === 'SCHEDULE' && (
          <div>
            {schedules.length > 0 && (
              <div className="schedule-active-card">
                <div className="schedule-active-head">
                  <CheckCircle2 size={16} />
                  <strong>Schedule active</strong>
                </div>
                <p className="field-hint">
                  Cron: <code>{scheduleCron(schedules[0])}</code>
                  {scheduleNextRun(schedules[0]) && (
                    <> · <strong>Next run</strong> {formatCronTime?.(scheduleNextRun(schedules[0])) || scheduleNextRun(schedules[0])}</>
                  )}
                </p>
                <p className="field-hint schedule-next-hint">
                  Updates automatically after each scheduled run (every ~10s).
                </p>
                <p className="field-hint">
                  Open <strong>Results</strong> — output appears automatically when cron fires (no Run button needed).
                </p>
                <button
                  type="button"
                  className="btn-sm btn-danger-outline"
                  onClick={() => onStopSchedule?.(parseScheduleId(schedules[0]))}
                >
                  Stop schedule
                </button>
              </div>
            )}
            <Field
              label="Cron expression"
              hint="Five fields: minute hour day month weekday. Example: */5 * * * * = every 5 minutes."
            >
              <input
                value={cfg.cronExpression !== undefined ? cfg.cronExpression : '0 9 * * *'}
                onChange={(e) => onChange('cronExpression', e.target.value)}
                placeholder="*/5 * * * *"
                spellCheck={false}
              />
              <div className="cron-presets">
                <button type="button" className="btn-xs" onClick={() => onChange('cronExpression', '*/1 * * * *')}>
                  Every 1 min
                </button>
                <button type="button" className="btn-xs" onClick={() => onChange('cronExpression', '*/5 * * * *')}>
                  Every 5 min
                </button>
                <button type="button" className="btn-xs" onClick={() => onChange('cronExpression', '0 * * * *')}>
                  Every hour
                </button>
                <button type="button" className="btn-xs" onClick={() => onChange('cronExpression', '0 9 * * *')}>
                  Daily 9:00
                </button>
              </div>
            </Field>
            <button
              type="button"
              className="btn-sm btn-accent"
              onClick={() => {
                const expr = (cfg.cronExpression ?? '0 9 * * *').trim();
                if (!expr) {
                  toast?.('Enter a cron expression (e.g. */5 * * * *)', 'error');
                  return;
                }
                onCreateSchedule(expr);
              }}
              disabled={!canCreateWebhook}
            >
              {schedules.length ? 'Update schedule' : 'Start schedule'}
            </button>
            {!canCreateWebhook && (
              <p className="field-hint" style={{ marginTop: '0.5rem' }}>
                Save the workflow first, then start the schedule.
              </p>
            )}
          </div>
        )}

        <LastRunFooter lastRun={lastRun} />
      </div>
    );
  }

  const catalog = NODE_CATALOG[type];
  if (!catalog) {
    return <div className="empty-state">Unknown node type.</div>;
  }

  const createCred = async () => {
    setCredError('');
    try {
      const c = await credApi.createCredential({ name: credName, apiKey: credKey });
      const cid = parseCredId(c);
      onChange('credentialId', cid);
      setShowCredModal(false);
      setCredKey('');
      const list = await credApi.listCredentials();
      setCredentials(list || []);
    } catch (e) {
      setCredError(e.message || 'Failed to save credential');
    }
  };

  return (
    <div className="node-config-intro">
      <Field label="Display name">
        <input value={node.data.label || ''} onChange={(e) => onRename(e.target.value)} />
      </Field>
      <h4 className="config-heading">{catalog.name}</h4>
      <p className="config-desc">{catalog.description}</p>
      {catalog.example && (
        <p className="field-hint config-example"><strong>Example:</strong> {catalog.example}</p>
      )}

      {type === 'HTTP' && (
        <button
          type="button"
          className="btn-sm"
          style={{ marginBottom: '0.75rem' }}
          onClick={() => {
            onChange('method', 'GET');
            onChange('url', 'https://jsonplaceholder.typicode.com/todos/1');
            onChange('body', null);
          }}
        >
          Try public demo API (GET)
        </button>
      )}

      {type === 'TRANSFORM' && (
        <div className="transform-helpers">
          <p className="field-hint">
            Node id for expressions: <code>{node.id}</code>
          </p>
          <button
            type="button"
            className="btn-sm"
            style={{ marginBottom: '0.75rem' }}
            onClick={() => {
              if (!mapFromPreviousHttp()) {
                toast?.('Connect an HTTP Request into this Transform first', 'error');
              } else {
                toast?.('Mapped status and body from the previous HTTP step', 'success');
              }
            }}
          >
            Map from previous HTTP
          </button>
        </div>
      )}

      {type === 'EMAIL' && (
        <p className="field-hint" style={{ marginBottom: '0.75rem', color: '#fbbf24' }}>
          Needs SendGrid — pick a credential below or set SENDGRID_API_KEY in .env.
        </p>
      )}

      {catalog.fields.map((f) => {
        if (f.credential) {
          return (
            <Field key={f.key} label={f.label} hint={f.hint}>
              <select
                value={cfg.credentialId || ''}
                onChange={(e) => {
                  if (e.target.value === '__new__') {
                    setShowCredModal(true);
                    return;
                  }
                  onChange('credentialId', e.target.value);
                }}
              >
                <option value="">Use server env SENDGRID_API_KEY</option>
                {credentials.map((c) => (
                  <option key={parseCredId(c)} value={parseCredId(c)}>
                    {c.name || c.Name} (SendGrid)
                  </option>
                ))}
                <option value="__new__">+ Create new credential…</option>
              </select>
              {cfg.credentialId ? (
                <p className="field-hint readiness-all-done">Credential connected</p>
              ) : (
                <p className="field-hint">Or set SENDGRID_API_KEY in .env as a fallback.</p>
              )}
            </Field>
          );
        }

        if (f.select) {
          return (
            <Field key={f.key} label={f.label} hint={f.hint}>
              <select value={cfg[f.key] || f.select[0]} onChange={(e) => onChange(f.key, e.target.value)}>
                {f.select.map((m) => (
                  <option key={m} value={m}>{m}</option>
                ))}
              </select>
            </Field>
          );
        }

        if (f.textarea) {
          let raw = '';
          if (f.json) {
            if (f.key === 'body') {
              raw = typeof cfg.body === 'string' ? cfg.body : JSON.stringify(cfg.body ?? null, null, 2);
            } else {
              raw = JSON.stringify(cfg[f.key] ?? {}, null, 2);
            }
          } else {
            raw = cfg[f.key] || '';
          }
          return (
            <Field key={f.key} label={f.label} hint={f.hint}>
              <textarea
                rows={f.rows || 4}
                placeholder={f.placeholder}
                value={raw}
                onChange={(e) => {
                  if (f.json) {
                    if (f.key === 'body') {
                      try {
                        onChange(f.key, JSON.parse(e.target.value));
                      } catch {
                        onChange(f.key, e.target.value);
                      }
                    } else {
                      try {
                        onChange(f.key, JSON.parse(e.target.value));
                      } catch {
                        /* wait */
                      }
                    }
                  } else {
                    onChange(f.key, e.target.value);
                  }
                }}
              />
            </Field>
          );
        }

        return (
          <Field key={f.key} label={f.label} hint={f.hint}>
            <input
              value={cfg[f.key] || ''}
              placeholder={f.placeholder}
              onChange={(e) => onChange(f.key, e.target.value)}
            />
          </Field>
        );
      })}

      <LastRunFooter lastRun={lastRun} />

      {showCredModal && (
        <div className="modal-overlay" onClick={() => setShowCredModal(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>New SendGrid credential</h3>
            <Field label="Name">
              <input value={credName} onChange={(e) => setCredName(e.target.value)} />
            </Field>
            <Field label="API key" hint="From SendGrid → Settings → API Keys. Stored encrypted; never shown in the workflow.">
              <input
                type="password"
                value={credKey}
                onChange={(e) => setCredKey(e.target.value)}
                placeholder="SG...."
                autoComplete="off"
              />
            </Field>
            {credError && <p className="field-hint" style={{ color: '#fca5a5' }}>{credError}</p>}
            <div className="modal-actions">
              <button type="button" onClick={() => setShowCredModal(false)}>Cancel</button>
              <button type="button" className="btn-primary" onClick={createCred}>Save credential</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function LastRunFooter({ lastRun }) {
  if (!lastRun?.status || lastRun.status === 'idle') {
    if (lastRun?.output === undefined || lastRun?.output === null) return null;
  }
  const s = String(lastRun?.status || '').toUpperCase();
  const failed = s === 'ERROR' || s === 'FAILED';
  return (
    <div className={`last-run-footer ${failed ? 'failed' : 'ok'}`}>
      {s && s !== 'IDLE' && (
        failed ? (
          <p><strong>Last run failed</strong>{lastRun.error ? `: ${lastRun.error}` : ` (${s})`}</p>
        ) : (
          <p><strong>Last run:</strong> {s}</p>
        )
      )}
      {lastRun?.output !== undefined && lastRun?.output !== null && (
        <>
          <p className="field-hint" style={{ marginTop: '0.35rem' }}><strong>Output from this step</strong></p>
          <pre className="result-json">{JSON.stringify(lastRun.output, null, 2)}</pre>
        </>
      )}
    </div>
  );
}

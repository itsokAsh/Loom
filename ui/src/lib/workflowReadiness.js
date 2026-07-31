import { isStartType } from './nodeCatalog';

export function isNodeConfigured(node) {
  const type = node?.data?.nodeType;
  const cfg = node?.data?.config || {};
  if (type === 'EMAIL') {
    return Boolean(
      cfg.to?.trim() &&
        cfg.from?.trim() &&
        cfg.subject?.trim() &&
        cfg.body?.trim()
    );
  }
  if (type === 'HTTP') {
    return Boolean(cfg.method && cfg.url?.trim());
  }
  if (type === 'TRANSFORM') {
    return cfg.mapping && typeof cfg.mapping === 'object' && Object.keys(cfg.mapping).length > 0;
  }
  if (type === 'SCHEDULE') {
    return Boolean(cfg.cronExpression?.trim());
  }
  if (isStartType(type)) return true;
  return true;
}

export function getReadinessChecks({ nodes, activeId, dirty, webhooks, schedules, hasRun }) {
  const start = nodes.find((n) => isStartType(n.data?.nodeType));
  const actions = nodes.filter((n) => !isStartType(n.data?.nodeType));
  const needsWebhook = start?.data?.nodeType === 'WEBHOOK';
  const needsSchedule = start?.data?.nodeType === 'SCHEDULE';

  return [
    {
      id: 'start',
      label: 'Add a Start node',
      ok: Boolean(start),
      action: 'build',
      fix: 'From the left: Manual Trigger, Webhook, or Schedule.',
    },
    {
      id: 'steps',
      label: 'Add what the workflow does',
      ok: actions.length > 0,
      action: 'build',
      fix: 'Add HTTP Request, Send Email, or Transform — not another Start/Webhook.',
    },
    {
      id: 'saved',
      label: 'Save workflow',
      ok: Boolean(activeId) && !dirty,
      action: 'save',
      fix: 'Click Save in the top bar.',
    },
    ...(needsSchedule
      ? [
          {
            id: 'schedule',
            label: 'Start cron timer',
            ok: (schedules || []).length > 0,
            action: 'parameters',
            fix: 'Select Schedule → set cron → Save schedule.',
          },
          {
            id: 'results',
            label: 'See a scheduled run in Results',
            ok: Boolean(hasRun),
            action: 'results',
            fix: 'Open Results and wait — runs appear automatically.',
          },
        ]
      : [
          {
            id: 'test',
            label: 'Run the workflow',
            ok: Boolean(hasRun),
            action: 'test',
            fix: 'After Save, press Run (top bar). Then open Results to see JSON from each step.',
          },
        ]),
    ...(needsWebhook
      ? [
          {
            id: 'webhook',
            label: 'Create webhook URL',
            ok: (webhooks || []).length > 0,
            action: 'parameters',
            fix: 'Select the Webhook node, then Create URL.',
          },
        ]
      : []),
  ];
}

import { Mail, Globe, Shuffle, Play, Webhook, Clock } from 'lucide-react';

export const START_TYPES = new Set(['MANUAL', 'WEBHOOK', 'SCHEDULE']);

export const NODE_CATALOG = {
  MANUAL: {
    type: 'MANUAL',
    name: 'Manual Trigger',
    subtitle: 'Start · Test button',
    description: 'Run this workflow yourself from the Test tab. Best for learning.',
    category: 'Start',
    Icon: Play,
    color: '#f59e0b',
    isStart: true,
  },
  WEBHOOK: {
    type: 'WEBHOOK',
    name: 'Webhook',
    subtitle: 'Start · Incoming HTTP',
    description: 'When an app POSTs JSON to a URL, this workflow runs. Optional — only if an external app should start it.',
    category: 'Start',
    Icon: Webhook,
    color: '#f59e0b',
    isStart: true,
  },
  SCHEDULE: {
    type: 'SCHEDULE',
    name: 'Schedule',
    subtitle: 'Start · Cron',
    description: 'Run on a timer (e.g. every day at 9am).',
    category: 'Start',
    Icon: Clock,
    color: '#f59e0b',
    isStart: true,
  },
  EMAIL: {
    type: 'EMAIL',
    name: 'Send Email',
    subtitle: 'Action · SendGrid',
    description: 'Needs SendGrid (credential or SENDGRID_API_KEY). Set From, To, subject, and body.',
    example: 'To: {{trigger.email}} · From: your verified sender',
    category: 'Actions',
    Icon: Mail,
    color: '#10b981',
    fields: [
      {
        key: 'credentialId',
        label: 'SendGrid credential',
        hint: 'Create or select a SendGrid API key. Stored encrypted — never in the workflow.',
        credential: true,
      },
      {
        key: 'to',
        label: 'To (recipient)',
        hint: 'Who receives the email. Use {{trigger.email}} from your Test JSON.',
        placeholder: '{{trigger.email}}',
      },
      {
        key: 'from',
        label: 'From (sender address)',
        hint: 'Fixed email verified in SendGrid — cannot use trigger data.',
        placeholder: 'noreply@yourdomain.com',
      },
      {
        key: 'subject',
        label: 'Subject',
        hint: 'You can use {{trigger.name}} and other placeholders.',
        placeholder: 'Welcome, {{trigger.name}}!',
      },
      {
        key: 'body',
        label: 'Message body',
        hint: 'Plain text body of the email.',
        placeholder: 'Hi {{trigger.name}},\n\nWelcome!',
        textarea: true,
        rows: 6,
      },
    ],
  },
  HTTP: {
    type: 'HTTP',
    name: 'HTTP Request',
    subtitle: 'Action · Call any API',
    description: 'Calls a public URL. On success stores { status, body } for later steps (e.g. Transform).',
    example: 'GET https://jsonplaceholder.typicode.com/todos/1',
    category: 'Actions',
    Icon: Globe,
    color: '#3b82f6',
    fields: [
      {
        key: 'method',
        label: 'Method',
        hint: 'GET for reading data, POST for sending JSON.',
        select: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'],
      },
      {
        key: 'url',
        label: 'URL',
        hint: 'Full public URL. Fixed text only — cannot use {{trigger.*}} here. Localhost is blocked.',
        placeholder: 'https://jsonplaceholder.typicode.com/todos/1',
      },
      {
        key: 'body',
        label: 'Request body (JSON)',
        hint: 'Only for POST/PUT/PATCH. Leave empty for GET.',
        placeholder: '{"name": "{{trigger.name}}"}',
        textarea: true,
        rows: 6,
        json: true,
      },
    ],
  },
  TRANSFORM: {
    type: 'TRANSFORM',
    name: 'Transform Data',
    subtitle: 'Data · Shape JSON',
    description:
      'Builds a JSON object in memory for later steps — not a downloadable file. Use {{trigger.*}} or {{outputs.<nodeId>.*}}.',
    example: '{ "greeting": "Hello {{trigger.name}}" }',
    category: 'Data',
    Icon: Shuffle,
    color: '#8b5cf6',
    fields: [
      {
        key: 'mapping',
        label: 'Output mapping (JSON)',
        hint: 'Static JSON, or expressions like {{trigger.name}} / {{outputs.http_xxx.body}}.',
        placeholder: '{\n  "greeting": "Hello {{trigger.name}}"\n}',
        textarea: true,
        rows: 10,
        json: true,
      },
    ],
  },
};

export const PALETTE_SECTIONS = [
  { title: 'Start', types: ['MANUAL', 'WEBHOOK', 'SCHEDULE'] },
  { title: 'Actions', types: ['HTTP', 'EMAIL'] },
  { title: 'Data', types: ['TRANSFORM'] },
];

export function isStartType(type) {
  return START_TYPES.has(type) || type === 'TRIGGER';
}

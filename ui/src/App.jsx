import React, { useState, useRef, useEffect } from 'react';
import ReactFlow, {
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  MarkerType
} from 'reactflow';
import 'reactflow/dist/style.css';
import {
  Activity,
  UserPlus,
  MousePointerClick,
  CloudOff,
  RefreshCcw,
  Server,
  ShieldCheck,
  Fingerprint,
  Layers,
  Mail,
  Archive,
  Inbox,
  MailCheck,
  ShieldAlert,
  Clock,
  Terminal,
  Crosshair
} from 'lucide-react';

/* ─────────────────────── DAG DEFINITIONS ─────────────────────── */

const NODE_STYLE_BASE = {
  background: '#111111',
  borderColor: 'rgba(255,255,255,0.08)',
  color: '#f3f4f6',
  borderRadius: '14px',
  borderWidth: '2px',
  padding: '0',
  transition: 'border-color 0.4s ease, box-shadow 0.4s ease',
  fontSize: '13px',
  width: 180,
};

const makeLabel = (Icon, title, subtitle) => (
  <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 14px' }}>
    <Icon size={18} style={{ flexShrink: 0, opacity: 0.7 }} />
    <div>
      <div style={{ fontWeight: 600, lineHeight: 1.3 }}>{title}</div>
      {subtitle && <div style={{ fontSize: 11, opacity: 0.5, lineHeight: 1.3 }}>{subtitle}</div>}
    </div>
  </div>
);

const buildInitialNodes = () => [
  { id: 'backend',  type: 'input', position: { x: 220, y: 0 },   data: { label: makeLabel(Server, 'Your Backend', 'API Server') },  style: { ...NODE_STYLE_BASE } },
  { id: 'security', position: { x: 220, y: 100 }, data: { label: makeLabel(ShieldCheck, 'Security Gate', 'HMAC-SHA256') }, style: { ...NODE_STYLE_BASE } },
  { id: 'dedup',    position: { x: 220, y: 200 }, data: { label: makeLabel(Fingerprint, 'Duplicate Shield', 'Idempotency Lock') }, style: { ...NODE_STYLE_BASE } },
  { id: 'queue',    position: { x: 220, y: 300 }, data: { label: makeLabel(Layers, 'Message Queue', 'RabbitMQ') }, style: { ...NODE_STYLE_BASE } },
  { id: 'email',    position: { x: 100, y: 410 }, data: { label: makeLabel(Mail, 'Email Service', 'SendGrid API') }, style: { ...NODE_STYLE_BASE } },
  { id: 'dlq',      type: 'output', position: { x: 360, y: 410 }, data: { label: makeLabel(Archive, 'Safety Net', 'Dead Letter Queue') }, style: { ...NODE_STYLE_BASE, borderStyle: 'dashed' } },
];

const buildInitialEdges = () => [
  { id: 'e1', source: 'backend',  target: 'security', style: { stroke: 'rgba(255,255,255,0.1)' }, markerEnd: { type: MarkerType.ArrowClosed, color: 'rgba(255,255,255,0.1)' } },
  { id: 'e2', source: 'security', target: 'dedup',    style: { stroke: 'rgba(255,255,255,0.1)' }, markerEnd: { type: MarkerType.ArrowClosed, color: 'rgba(255,255,255,0.1)' } },
  { id: 'e3', source: 'dedup',    target: 'queue',    style: { stroke: 'rgba(255,255,255,0.1)' }, markerEnd: { type: MarkerType.ArrowClosed, color: 'rgba(255,255,255,0.1)' } },
  { id: 'e4', source: 'queue',    target: 'email',    style: { stroke: 'rgba(255,255,255,0.1)' }, markerEnd: { type: MarkerType.ArrowClosed, color: 'rgba(255,255,255,0.1)' } },
  { id: 'e5', source: 'email',    target: 'dlq',      label: 'retry failed', style: { stroke: 'rgba(255,255,255,0.06)' }, markerEnd: { type: MarkerType.ArrowClosed, color: 'rgba(255,255,255,0.06)' } },
];

/* ─────────────────────── MAIN APP ─────────────────────── */

function App() {
  const [nodes, setNodes, onNodesChange] = useNodesState(buildInitialNodes());
  const [edges, setEdges, onEdgesChange] = useEdgesState(buildInitialEdges());
  const [logs, setLogs] = useState([]);
  const [emails, setEmails] = useState([]);
  const [stats, setStats] = useState({ delivered: 0, blocked: 0, queued: 0 });
  const [comparison, setComparison] = useState(null);
  const [isSimulating, setIsSimulating] = useState(false);
  const [activeScenario, setActiveScenario] = useState(null);
  const [step, setStep] = useState(null);
  const logsEndRef = useRef(null);

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  /* ── Helpers ── */

  const wait = (ms) => new Promise(resolve => setTimeout(resolve, ms));

  const glowNode = (nodeId, color) => {
    setNodes(nds => nds.map(n =>
      n.id === nodeId
        ? { ...n, style: { ...n.style, borderColor: color, boxShadow: `0 0 20px ${color}33` } }
        : n
    ));
  };

  const dimNode = (nodeId) => {
    setNodes(nds => nds.map(n =>
      n.id === nodeId
        ? { ...n, style: { ...n.style, borderColor: 'rgba(255,255,255,0.08)', boxShadow: 'none' } }
        : n
    ));
  };

  const glowEdge = (edgeId, color) => {
    setEdges(eds => eds.map(e =>
      e.id === edgeId
        ? { ...e, animated: true, style: { stroke: color, strokeWidth: 2.5 }, markerEnd: { type: MarkerType.ArrowClosed, color } }
        : e
    ));
  };

  const addLog = (title, message, status) => {
    const ts = new Date().toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
    setLogs(prev => [...prev, { id: Date.now() + Math.random(), ts, title, message, status }]);
  };

  const addEmail = (type, subject, preview) => {
    setEmails(prev => [...prev, { id: Date.now() + Math.random(), type, subject, preview }]);
    setStats(prev => ({
      ...prev,
      delivered: prev.delivered + (type === 'delivered' ? 1 : 0),
      blocked: prev.blocked + (type === 'blocked' || type === 'alert' ? 1 : 0),
      queued: prev.queued + (type === 'queued' ? 1 : 0),
    }));
  };

  const resetAll = () => {
    setNodes(buildInitialNodes());
    setEdges(buildInitialEdges());
    setLogs([]);
    setEmails([]);
    setStats({ delivered: 0, blocked: 0, queued: 0 });
    setComparison(null);
    setStep(null);
    setActiveScenario(null);
  };

  /* ── Scenario 1: Normal Signup ── */

  const simulateSignup = async () => {
    const total = 5;

    setStep({ current: 1, total, label: 'Receiving request...' });
    glowNode('backend', '#3b82f6');
    addLog('📤 Request Received', 'Your backend just called Loom: "Send a welcome email to sarah@example.com"', 'info');
    await wait(1800);

    setStep({ current: 2, total, label: 'Verifying security...' });
    glowEdge('e1', '#3b82f6');
    glowNode('security', '#3b82f6');
    addLog('🔒 Security Gate', 'Loom checks the request\'s digital signature. This proves the request actually came from YOUR server — not an attacker trying to abuse the API.', 'info');
    await wait(600);
    addLog('', '✅ Signature valid. Request is authentic.', 'success');
    await wait(1400);

    setStep({ current: 3, total, label: 'Checking for duplicates...' });
    glowEdge('e2', '#3b82f6');
    glowNode('dedup', '#3b82f6');
    addLog('🛡️ Duplicate Shield', 'Loom writes a unique "fingerprint" for this request into the database. If it ever sees this exact fingerprint again, it will know it\'s a duplicate.', 'info');
    await wait(600);
    addLog('', '✅ First time seeing this request. Proceeding.', 'success');
    await wait(1400);

    setStep({ current: 4, total, label: 'Dispatching to worker...' });
    glowEdge('e3', '#10b981');
    glowNode('queue', '#10b981');
    addLog('📨 Message Queue', 'The task is placed on a reliable message queue (RabbitMQ). Even if your backend crashes right now, this email will still be delivered.', 'info');
    await wait(1800);

    setStep({ current: 5, total, label: 'Sending email...' });
    glowEdge('e4', '#10b981');
    glowNode('email', '#10b981');
    addLog('📧 Email Delivered!', 'Worker successfully sent the welcome email to sarah@example.com via SendGrid.', 'success');
    addEmail('delivered', 'Welcome to our platform! 🎉', 'Hi Sarah, thanks for signing up. We\'re excited to have you on board...');
    await wait(1200);

    setStep({ current: total, total, label: 'Complete!' });
    setComparison({
      without: 'If SendGrid was slow, your API would hang and the user would see a loading spinner.',
      with: 'Loom processed it asynchronously. Your API responded instantly. The email was delivered reliably in the background.',
    });
  };

  /* ── Scenario 2: Duplicate ── */

  const simulateDuplicate = async () => {
    const total = 7;

    setStep({ current: 1, total, label: 'First request arriving...' });
    glowNode('backend', '#3b82f6');
    addLog('📤 First Request', 'Your backend calls Loom: "Send a welcome email to sarah@example.com"', 'info');
    await wait(1200);

    setStep({ current: 2, total, label: 'Processing first request...' });
    glowEdge('e1', '#3b82f6');
    glowNode('security', '#10b981');
    glowEdge('e2', '#3b82f6');
    glowNode('dedup', '#10b981');
    addLog('🔒 → 🛡️', 'Security passed. Fingerprint recorded. This is the first time we\'ve seen this request.', 'success');
    await wait(1200);

    setStep({ current: 3, total, label: 'Delivering first email...' });
    glowEdge('e3', '#10b981');
    glowNode('queue', '#10b981');
    glowEdge('e4', '#10b981');
    glowNode('email', '#10b981');
    addLog('📧 Email Delivered', 'Welcome email sent successfully to sarah@example.com.', 'success');
    addEmail('delivered', 'Welcome to our platform! 🎉', 'Hi Sarah, thanks for signing up...');
    await wait(1800);

    // Reset graph for second request
    setNodes(buildInitialNodes());
    setEdges(buildInitialEdges());

    setStep({ current: 4, total, label: '⚠️ Duplicate arriving...' });
    addLog('', '─── 2 seconds later ───', 'muted');
    await wait(1000);

    glowNode('backend', '#f59e0b');
    addLog('📤 Second Request!', 'The SAME request came in again — the user double-clicked the Register button!', 'warning');
    await wait(1800);

    setStep({ current: 5, total, label: 'Security check (again)...' });
    glowEdge('e1', '#f59e0b');
    glowNode('security', '#f59e0b');
    addLog('🔒 Security Gate', 'Signature is still valid — this IS from your backend, not an attacker. The problem is the request is a duplicate.', 'info');
    await wait(1800);

    setStep({ current: 6, total, label: 'Duplicate detected!' });
    glowEdge('e2', '#ef4444');
    glowNode('dedup', '#ef4444');
    addLog('🛡️ BLOCKED!', 'Loom\'s Duplicate Shield caught it! The database recognized this fingerprint — this exact request was already processed 2 seconds ago.', 'danger');
    await wait(800);
    addLog('', '🚫 Second email PREVENTED. Sarah will only receive one welcome email.', 'danger');
    addEmail('blocked', 'Welcome to our platform! 🎉', 'BLOCKED — duplicate request prevented by Loom');
    await wait(1500);

    setStep({ current: 7, total, label: 'Complete!' });
    setComparison({
      without: 'Sarah would receive 2 identical welcome emails. Unprofessional and confusing.',
      with: 'Sarah receives exactly 1 email. The duplicate was silently caught and dropped — she\'ll never know it happened.',
    });
  };

  /* ── Scenario 3: Failure + DLQ Recovery ── */

  const simulateFailure = async () => {
    const total = 11;

    setStep({ current: 1, total, label: 'Receiving request...' });
    glowNode('backend', '#3b82f6');
    addLog('📤 Request Received', 'Your backend calls Loom: "Send a welcome email to sarah@example.com"', 'info');
    await wait(1200);

    setStep({ current: 2, total, label: 'Verifying & checking...' });
    glowEdge('e1', '#3b82f6');
    glowNode('security', '#10b981');
    glowEdge('e2', '#3b82f6');
    glowNode('dedup', '#10b981');
    addLog('🔒 → 🛡️', 'Security passed. Fingerprint recorded. Proceeding to execution.', 'success');
    await wait(1200);

    setStep({ current: 3, total, label: 'Dispatching to worker...' });
    glowEdge('e3', '#10b981');
    glowNode('queue', '#10b981');
    addLog('📨 Dispatched', 'Task placed on the message queue and picked up by a worker.', 'info');
    await wait(1500);

    // Attempt 1
    setStep({ current: 4, total, label: 'Attempt 1 of 3...' });
    glowEdge('e4', '#ef4444');
    glowNode('email', '#ef4444');
    addLog('❌ Attempt 1/3 Failed', 'SendGrid returned 429 Too Many Requests. The email API is overloaded right now.', 'danger');
    await wait(1000);
    addLog('⏳ Retrying...', 'Loom waits 2 seconds before trying again (exponential backoff).', 'muted');
    await wait(1500);

    // Attempt 2
    setStep({ current: 5, total, label: 'Attempt 2 of 3...' });
    glowNode('email', '#ef4444');
    addLog('❌ Attempt 2/3 Failed', 'SendGrid is still down. Error: 429 Too Many Requests.', 'danger');
    await wait(1000);
    addLog('⏳ Retrying...', 'Loom waits 4 seconds (backoff doubles each time).', 'muted');
    await wait(1500);

    // Attempt 3
    setStep({ current: 6, total, label: 'Attempt 3 of 3...' });
    glowNode('email', '#ef4444');
    addLog('❌ Attempt 3/3 Failed', 'SendGrid is still unavailable. All retry attempts exhausted.', 'danger');
    await wait(1800);

    // Route to DLQ
    setStep({ current: 7, total, label: 'Routing to Safety Net...' });
    glowEdge('e5', '#f59e0b');
    glowNode('dlq', '#f59e0b');
    addLog('🗄️ Saved to Safety Net', 'Instead of losing the email forever, Loom stores it in the Dead Letter Queue. The task is safely preserved with all its data.', 'warning');
    addEmail('queued', 'Welcome to our platform! 🎉', 'Queued in Dead Letter Queue — waiting for provider to recover...');
    await wait(2000);

    // ── Recovery Phase ──
    addLog('', '─── 3 minutes later ───', 'muted');
    await wait(1500);

    setStep({ current: 8, total, label: 'Provider recovering...' });
    dimNode('dlq');
    dimNode('email');
    addLog('📡 Provider Status Change', 'SendGrid\'s status page reports: services are recovering. API is accepting requests again.', 'info');
    await wait(1800);

    setStep({ current: 9, total, label: 'Picking up from Safety Net...' });
    glowNode('dlq', '#3b82f6');
    addLog('🔄 DLQ Retry Triggered', 'Loom detects the provider is healthy again. Picking up the stored task from the Dead Letter Queue...', 'info');
    await wait(1500);

    setStep({ current: 10, total, label: 'Retrying delivery...' });
    glowNode('dlq', '#10b981');
    glowNode('email', '#10b981');
    addLog('📧 Email Delivered!', 'The task from the Safety Net was successfully re-executed. Welcome email delivered to sarah@example.com!', 'success');

    // Transform the queued email into a delivered one
    setEmails(prev => prev.map(em =>
      em.type === 'queued'
        ? { ...em, type: 'delivered', preview: '✅ Recovered from Safety Net and delivered successfully!' }
        : em
    ));
    setStats(prev => ({ ...prev, delivered: prev.delivered + 1, queued: prev.queued - 1 }));
    await wait(1500);

    setStep({ current: 11, total, label: 'Complete!' });
    setComparison({
      without: 'The email silently disappears. Sarah never gets her welcome email. Nobody even knows it failed.',
      with: 'Loom caught the failure, retried 3 times, safely stored it, and automatically delivered it once SendGrid recovered. Zero data loss.',
    });
  };

  /* ── Scenario 4: SSRF Attack ── */

  const simulateSSRF = async () => {
    const total = 7;

    setStep({ current: 1, total, label: 'Malicious request arriving...' });
    glowNode('backend', '#ef4444');
    addLog('🎯 Suspicious Request', 'A workflow has been triggered. But the HTTP webhook URL in the payload points to http://169.254.169.254/latest/meta-data/ — the AWS metadata endpoint that stores your server\'s secret credentials.', 'danger');
    await wait(2200);

    setStep({ current: 2, total, label: 'Checking signature...' });
    glowEdge('e1', '#f59e0b');
    glowNode('security', '#f59e0b');
    addLog('🔒 Security Gate', 'The HMAC signature is valid — this request DID come from an authenticated source. But the payload inside contains a dangerous URL.', 'warning');
    await wait(600);
    addLog('', '⚠️ Signature check alone cannot detect SSRF. This is why Loom has a second layer of defense...', 'muted');
    await wait(2000);

    setStep({ current: 3, total, label: 'Fingerprint check...' });
    glowEdge('e2', '#f59e0b');
    glowNode('dedup', '#f59e0b');
    addLog('🛡️ Duplicate Shield', 'This is a new request. Fingerprint recorded. Passing to execution.', 'info');
    await wait(1500);

    setStep({ current: 4, total, label: 'Dispatching to worker...' });
    glowEdge('e3', '#f59e0b');
    glowNode('queue', '#f59e0b');
    addLog('📨 Dispatched', 'Task queued for execution. The worker picks it up and prepares to make the HTTP request.', 'info');
    await wait(1800);

    setStep({ current: 5, total, label: 'Inspecting target URL...' });
    glowEdge('e4', '#ef4444');
    glowNode('email', '#ef4444');
    addLog('🔍 URL Inspection', 'Before making the HTTP request, Loom\'s hardened HTTP client resolves the target URL and checks the IP address against a blocklist:', 'info');
    await wait(800);
    addLog('', '→ Blocked ranges: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16, 127.0.0.0/8', 'muted');
    await wait(800);
    addLog('', '→ Target URL http://169.254.169.254 resolves to IP 169.254.169.254', 'muted');
    await wait(800);
    addLog('', '→ IP 169.254.169.254 falls in the 169.254.0.0/16 link-local range... MATCH!', 'danger');
    await wait(1500);

    setStep({ current: 6, total, label: '🚨 SSRF BLOCKED!' });
    glowNode('email', '#ef4444');
    addLog('🚨 SSRF ATTACK BLOCKED', 'Loom refused to execute the request! The target URL resolves to a private IP address — this is a classic Server-Side Request Forgery (SSRF) attack. The attacker was trying to steal your cloud credentials from the AWS metadata endpoint.', 'danger');
    addEmail('alert', '🚨 SSRF Attack Intercepted', 'Attempt to access 169.254.169.254 (AWS metadata) was blocked. Your cloud credentials are safe.');
    await wait(2000);

    setStep({ current: 7, total, label: 'Complete!' });
    setComparison({
      without: 'The attacker steals your AWS IAM credentials, secret keys, and session tokens from the metadata endpoint. Full cloud account compromise in seconds.',
      with: 'The request never left the server. Loom\'s IP blocklist intercepted it before any network call was made. Your infrastructure is completely protected.',
    });
  };

  const runSimulation = async (scenario) => {
    if (isSimulating) return;
    setIsSimulating(true);
    setActiveScenario(scenario);
    resetAll();
    setActiveScenario(scenario);

    try {
      if (scenario === 'signup') await simulateSignup();
      else if (scenario === 'duplicate') await simulateDuplicate();
      else if (scenario === 'failure') await simulateFailure();
      else if (scenario === 'ssrf') await simulateSSRF();
    } catch (e) {
      console.error(e);
    }

    setIsSimulating(false);
  };

  /* ── Render ── */

  return (
    <div className="app-container">

      {/* ═══ LEFT SIDEBAR ═══ */}
      <div className="sidebar">
        <div className="sidebar-header">
          <h2 className="text-gradient flex-row gap-2" style={{ fontSize: '1.1rem' }}>
            <Activity size={22} /> Loom Engine
          </h2>
          <div className="text-muted" style={{ fontSize: '0.78rem', marginTop: 4 }}>Interactive Backend Simulator</div>
        </div>

        {/* Scenario Cards */}
        <div style={{ padding: '1rem 1.25rem', borderBottom: '1px solid var(--border-subtle)', display: 'flex', flexDirection: 'column', gap: '0.6rem', overflowY: 'auto', maxHeight: '340px' }}>
          <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--text-dark)', fontWeight: 600, marginBottom: 2 }}>Choose a Scenario</div>

          <button className={`scenario-card ${activeScenario === 'signup' ? 'active' : ''}`} disabled={isSimulating} onClick={() => runSimulation('signup')}>
            <div className="scenario-icon green"><UserPlus size={18} /></div>
            <div>
              <div style={{ fontWeight: 600, fontSize: '0.85rem', color: 'var(--text-main)' }}>A New User Signs Up</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', lineHeight: 1.4 }}>Watch Loom deliver a welcome email reliably</div>
            </div>
          </button>

          <button className={`scenario-card ${activeScenario === 'duplicate' ? 'active' : ''}`} disabled={isSimulating} onClick={() => runSimulation('duplicate')}>
            <div className="scenario-icon blue"><MousePointerClick size={18} /></div>
            <div>
              <div style={{ fontWeight: 600, fontSize: '0.85rem', color: 'var(--text-main)' }}>User Double-Clicks Register</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', lineHeight: 1.4 }}>See how Loom prevents duplicate emails</div>
            </div>
          </button>

          <button className={`scenario-card ${activeScenario === 'failure' ? 'active' : ''}`} disabled={isSimulating} onClick={() => runSimulation('failure')}>
            <div className="scenario-icon amber"><CloudOff size={18} /></div>
            <div>
              <div style={{ fontWeight: 600, fontSize: '0.85rem', color: 'var(--text-main)' }}>Email Provider Goes Down</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', lineHeight: 1.4 }}>Watch Loom handle failure and auto-recover</div>
            </div>
          </button>

          <button className={`scenario-card ${activeScenario === 'ssrf' ? 'active' : ''}`} disabled={isSimulating} onClick={() => runSimulation('ssrf')}>
            <div className="scenario-icon red"><Crosshair size={18} /></div>
            <div>
              <div style={{ fontWeight: 600, fontSize: '0.85rem', color: 'var(--text-main)' }}>Attacker Tries SSRF</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', lineHeight: 1.4 }}>Watch Loom block an attack on your cloud credentials</div>
            </div>
          </button>

          {(isSimulating || logs.length > 0) && (
            <button className="scenario-card" onClick={resetAll} disabled={isSimulating} style={{ justifyContent: 'center', opacity: isSimulating ? 0.3 : 0.6 }}>
              <RefreshCcw size={14} style={{ opacity: 0.5 }} />
              <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Reset</span>
            </button>
          )}
        </div>

        {/* Decision Trace */}
        <div style={{ padding: '0.75rem 1.25rem 0', display: 'flex', alignItems: 'center', gap: 6 }}>
          <Terminal size={14} style={{ color: 'var(--text-dark)' }} />
          <span style={{ fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--text-dark)', fontWeight: 600 }}>Decision Trace</span>
        </div>

        <div className="trace-panel">
          {logs.length === 0 ? (
            <div className="trace-empty">
              <Terminal size={32} />
              <div style={{ fontSize: '0.82rem' }}>Pick a scenario above to watch<br />the engine make decisions.</div>
            </div>
          ) : (
            logs.map(log => (
              <div className="trace-entry" key={log.id}>
                {log.ts && <div className="trace-time">{log.ts}</div>}
                <div className={`trace-body status-${log.status}`}>
                  {log.title && <div className="trace-title" style={{ color: `var(--accent-${log.status})` }}>{log.title}</div>}
                  <div className="trace-message">{log.message}</div>
                </div>
              </div>
            ))
          )}
          <div ref={logsEndRef} />
        </div>
      </div>

      {/* ═══ CENTER: DAG ═══ */}
      <div className="main-content">
        <div className="top-nav">
          <h3 style={{ fontSize: '0.95rem' }}>Workflow Execution Map</h3>
          <div className="flex-row gap-2 text-muted" style={{ fontSize: '0.82rem' }}>
            <span className={`status-dot ${isSimulating ? 'status-active' : 'status-success'}`}></span>
            {isSimulating ? 'Processing...' : 'Engine Idle'}
          </div>
        </div>

        <div className="content-area">
          {step && (
            <div className="step-indicator">
              <span className="step-label">Step {step.current}/{step.total}</span>
              <div className="step-progress-track">
                <div className="step-progress-fill" style={{ width: `${(step.current / step.total) * 100}%` }} />
              </div>
              <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>{step.label}</span>
            </div>
          )}

          <div className="glass-panel" style={{ flex: 1, position: 'relative', borderRadius: 'var(--radius-lg)', overflow: 'hidden', minHeight: 0 }}>
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              fitView
              fitViewOptions={{ padding: 0.3 }}
              attributionPosition="bottom-left"
              proOptions={{ hideAttribution: true }}
            >
              <Controls position="bottom-right" />
              <Background color="rgba(255,255,255,0.03)" gap={20} />
            </ReactFlow>
          </div>
        </div>
      </div>

      {/* ═══ RIGHT: INBOX ═══ */}
      <div className="inbox-panel">
        <div className="inbox-header">
          <div className="flex-row gap-2" style={{ fontSize: '0.95rem', fontWeight: 600 }}>
            <Inbox size={18} style={{ opacity: 0.6 }} /> sarah@example.com
          </div>
          <div className="text-muted" style={{ fontSize: '0.75rem', marginTop: 2 }}>Mock Email Inbox</div>
        </div>

        <div className="inbox-stats">
          <div className="inbox-stat">
            <div className="inbox-stat-value" style={{ color: 'var(--accent-success)' }}>{stats.delivered}</div>
            <div className="inbox-stat-label">Delivered</div>
          </div>
          <div className="inbox-stat">
            <div className="inbox-stat-value" style={{ color: stats.blocked > 0 ? 'var(--accent-danger)' : 'var(--text-dark)' }}>{stats.blocked}</div>
            <div className="inbox-stat-label">Blocked</div>
          </div>
        </div>

        <div className="inbox-emails">
          {emails.length === 0 ? (
            <div className="inbox-empty">
              <Mail size={32} />
              <div style={{ fontSize: '0.82rem' }}>No emails yet.<br />Run a simulation to see<br />what arrives in Sarah's inbox.</div>
            </div>
          ) : (
            emails.map(em => (
              <div className={`email-card ${em.type}`} key={em.id}>
                <div className={`email-badge ${em.type}`}>
                  {em.type === 'delivered' && <><MailCheck size={12} /> DELIVERED</>}
                  {em.type === 'blocked' && <><ShieldAlert size={12} /> BLOCKED</>}
                  {em.type === 'queued' && <><Clock size={12} /> QUEUED FOR RETRY</>}
                  {em.type === 'alert' && <><ShieldAlert size={12} /> SECURITY ALERT</>}
                </div>
                <div className="email-from">From: Your App via Loom</div>
                <div className="email-subject">{em.subject}</div>
                <div className="email-preview">{em.preview}</div>
              </div>
            ))
          )}
        </div>

        {comparison && (
          <div className="comparison-banner">
            <div className="comparison-row without">
              <div className="comparison-label without">🚫 Without Loom</div>
              <div style={{ fontSize: '0.82rem', color: 'var(--text-muted)' }}>{comparison.without}</div>
            </div>
            <div className="comparison-row with">
              <div className="comparison-label with">✅ With Loom</div>
              <div style={{ fontSize: '0.82rem', color: 'var(--text-muted)' }}>{comparison.with}</div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default App;

import React, { useCallback, useEffect, useRef, useState } from 'react';
import ReactFlow, {
  Background,
  Controls,
  addEdge,
  useEdgesState,
  useNodesState,
  MarkerType,
  ReactFlowProvider,
} from 'reactflow';
import 'reactflow/dist/style.css';
import {
  Plus,
  Save,
  Play,
  Download,
  Upload,
  Trash2,
  Copy,
  LayoutTemplate,
  List,
  AlertCircle,
} from 'lucide-react';
import { LoomNode, TriggerNode } from './nodes/LoomNodes';
import {
  loomDagToFlow,
  flowToLoomDag,
  defaultConfig,
  newNodeId,
  friendlyLabel,
  getStartNode,
} from '../lib/dagTransform';
import { validateFlowClient } from '../lib/dagValidate';
import { isStartType, NODE_CATALOG, PALETTE_SECTIONS } from '../lib/nodeCatalog';
import * as wfApi from '../api/workflows';
import * as tplApi from '../api/templates';
import * as execApi from '../api/executions';
import NodeConfigForm from './NodeConfigForm';
import WorkflowReadiness from './WorkflowReadiness';

const nodeTypes = { loom: LoomNode, trigger: TriggerNode };

const FEATURED = new Set([
  'call-an-api',
  'api-health-check',
  'webhook-relay',
  'welcome-email',
  'signup-dual-notify',
  'vip-conditional',
]);

const DEFAULT_MANUAL_TRIGGER = '{\n  "email": "you@example.com",\n  "name": "Ash"\n}';
const DEFAULT_SCHEDULE_TRIGGER = '{}';

function parseTriggerFromExecution(ex) {
  if (!ex) return null;
  const raw = ex.triggerData ?? ex.TriggerData;
  if (raw == null) return null;
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw);
    } catch {
      return null;
    }
  }
  if (typeof raw === 'object') return raw;
  return null;
}

function isScheduledExecution(ex, triggerData) {
  const idemp = ex?.idempotencyKey ?? ex?.IdempotencyKey ?? '';
  if (String(idemp).startsWith('cron-')) return true;
  return triggerData != null && triggerData.cron_time != null;
}

function formatCronTime(cronTime) {
  try {
    const d = new Date(cronTime);
    if (!Number.isNaN(d.getTime())) return d.toLocaleString();
  } catch {
    /* ignore */
  }
  return String(cronTime);
}

function parseId(obj) {
  if (!obj) return '';
  if (typeof obj === 'string') return obj;
  if (typeof obj === 'number') return String(obj);
  const raw = obj.id ?? obj.ID ?? obj.workflow_id ?? obj.workflowId ?? obj.execution_id ?? obj.executionId;
  if (raw == null) return '';
  if (typeof raw === 'string') return raw;
  if (raw?.String) return raw.String;
  // pgtype UUID sometimes serializes as { Bytes: [...], Valid: true }
  try {
    if (raw?.Bytes) {
      const b = Object.values(raw.Bytes).map((x) => Number(x));
      if (b.length === 16) {
        const hex = b.map((x) => x.toString(16).padStart(2, '0')).join('');
        return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
      }
    }
  } catch {
    /* ignore */
  }
  return String(raw);
}

function workflowDisplayName(name) {
  if (!name) return 'Untitled';
  return String(name)
    .replace(/→/g, ' to ')
    .replace(/\\rightarrow/g, ' to ')
    .replace(/\$rightarrow\$/gi, ' to ')
    .trim();
}

function workflowCreatedLabel(w) {
  const raw = w.createdAt || w.CreatedAt;
  if (!raw) return '';
  try {
    const d = new Date(raw);
    return d.toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return '';
  }
}

function sortWorkflows(list) {
  return [...(list || [])].sort((a, b) => {
    const ta = new Date(a.createdAt || a.CreatedAt || 0).getTime();
    const tb = new Date(b.createdAt || b.CreatedAt || 0).getTime();
    return tb - ta;
  });
}

function WorkflowBuilderInner({ toast }) {
  const reactFlowWrapper = useRef(null);
  const [rfInstance, setRfInstance] = useState(null);
  const [workflows, setWorkflows] = useState([]);
  const [templates, setTemplates] = useState([]);
  const [activeId, setActiveId] = useState(null);
  const [workflowMeta, setWorkflowMeta] = useState(null);
  const [version, setVersion] = useState(1);
  const [dirty, setDirty] = useState(false);
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [selectedNode, setSelectedNode] = useState(null);
  const [selectedEdge, setSelectedEdge] = useState(null);
  const [rightTab, setRightTab] = useState('parameters');
  const [sidebarMode, setSidebarMode] = useState('workflows');
  const [loading, setLoading] = useState(true);
  const [apiError, setApiError] = useState(null);
  const [webhooks, setWebhooks] = useState([]);
  const [schedules, setSchedules] = useState([]);
  const [executions, setExecutions] = useState([]);
  const [triggerPayload, setTriggerPayload] = useState(DEFAULT_MANUAL_TRIGGER);
  const [triggerInputKind, setTriggerInputKind] = useState(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [activeExecution, setActiveExecution] = useState(null);
  const [nodeStatuses, setNodeStatuses] = useState({});
  const [nodeErrors, setNodeErrors] = useState({});
  const [nodeOutputs, setNodeOutputs] = useState({});
  const [lastTriggerData, setLastTriggerData] = useState(null);
  const [validationErrors, setValidationErrors] = useState([]);
  const [running, setRunning] = useState(false);
  const [contextMenu, setContextMenu] = useState(null);
  const [tplConfig, setTplConfig] = useState({});
  const [tplModal, setTplModal] = useState(null);
  const executionsRef = useRef([]);
  const suspendDirtyUntilRef = useRef(0);
  const userDraggedNodeRef = useRef(false);

  const bumpSuspendDirty = useCallback((ms = 1500) => {
    suspendDirtyUntilRef.current = Date.now() + ms;
  }, []);

  const isDirtySuspended = useCallback(() => Date.now() < suspendDirtyUntilRef.current, []);

  useEffect(() => {
    executionsRef.current = executions;
  }, [executions]);

  useEffect(() => {
    if (!rfInstance || !activeId) return;
    bumpSuspendDirty(1200);
    const t = setTimeout(() => rfInstance.fitView({ padding: 0.15, duration: 0 }), 120);
    return () => clearTimeout(t);
  }, [activeId, rfInstance, bumpSuspendDirty]);

  const loadWorkflows = useCallback(async () => {
    try {
      const list = await wfApi.listWorkflows();
      setWorkflows(list || []);
      setApiError(null);
    } catch (e) {
      setApiError(e.message || 'Cannot reach Loom API');
    }
  }, []);

  const loadTemplates = useCallback(async () => {
    try {
      const res = await tplApi.listTemplates();
      setTemplates(res?.templates || res || []);
    } catch {
      /* optional */
    }
  }, []);

  useEffect(() => {
    (async () => {
      setLoading(true);
      await loadWorkflows();
      await loadTemplates();
      setLoading(false);
    })();
  }, [loadWorkflows, loadTemplates]);

  const blankCanvas = () => {
    setActiveId(null);
    setWorkflowMeta({ name: 'My Workflow' });
    setVersion(1);
    setNodes([]);
    setEdges([]);
    setDirty(false);
    setWebhooks([]);
    setSchedules([]);
    setExecutions([]);
    setSelectedNode(null);
    setSelectedEdge(null);
    setValidationErrors([]);
    setNodeStatuses({});
    setNodeErrors({});
    setNodeOutputs({});
    setLastTriggerData(null);
    setTriggerInputKind(null);
  };

  useEffect(() => {
    if (!loading && nodes.length === 0 && !activeId && !workflowMeta) {
      blankCanvas();
    }
  }, [loading]);

  useEffect(() => {
    if (validationErrors.length === 0) return;
    if (validateFlowClient(nodes, edges).length === 0) {
      setValidationErrors([]);
    }
  }, [nodes, edges, validationErrors.length]);

  const refreshExecutions = useCallback(async () => {
    if (!activeId) return [];
    const ex = await wfApi.listExecutions(activeId);
    setExecutions(ex || []);
    return ex || [];
  }, [activeId]);

  const refreshSchedules = useCallback(async () => {
    if (!activeId) return [];
    const sch = await wfApi.listSchedules(activeId);
    setSchedules(sch || []);
    return sch || [];
  }, [activeId]);

  const openWorkflow = async (id, opts = {}) => {
    try {
      const wf = await wfApi.getWorkflow(id);
      const wid = parseId(wf);
      setActiveId(wid);
      setWorkflowMeta(wf);
      setVersion(wf.version || 1);
      const flow = loomDagToFlow(wf.dag || { nodes: [], edges: [] }, nodeStatuses);
      setNodes(flow.nodes);
      setEdges(flow.edges);
      setDirty(false);
      bumpSuspendDirty();
      const wh = await wfApi.listWebhooks(wid);
      setWebhooks(wh || []);
      const sch = await wfApi.listSchedules(wid);
      setSchedules(sch || []);
      const ex = await wfApi.listExecutions(wid);
      setExecutions(ex || []);
      setSelectedNode(null);
      if (opts.sampleTrigger) {
        setTriggerPayload(JSON.stringify(opts.sampleTrigger, null, 2));
      } else {
        const start = flow.nodes.find((n) => isStartType(n.data?.nodeType));
        if (start?.data?.nodeType === 'SCHEDULE') {
          setTriggerPayload(DEFAULT_SCHEDULE_TRIGGER);
        }
      }
      setTriggerInputKind(null);
      const startNode = flow.nodes.find((n) => isStartType(n.data?.nodeType));
      if (startNode?.data?.nodeType === 'SCHEDULE' && (sch || []).length > 0) {
        setRightTab('executions');
      } else if (opts.openTest) {
        setRightTab('test');
      }
    } catch (e) {
      toast(e.message, 'error');
    }
  };

  const addNode = (type, position) => {
    if (isStartType(type) && getStartNode(nodes)) {
      toast('Only one Start node allowed. Delete the current one first.', 'error');
      return;
    }
    const id = newNodeId(type);
    const catalog = NODE_CATALOG[type];
    const pos = position || {
      x: isStartType(type) ? 220 : 160 + nodes.length * 28,
      y: isStartType(type) ? 60 : 220,
    };
    const newNode = {
      id,
      type: isStartType(type) ? 'trigger' : 'loom',
      position: pos,
      data: {
        label: catalog?.name || friendlyLabel(type),
        nodeType: type,
        config: defaultConfig(type),
        status: 'idle',
      },
    };
    setNodes((nds) => [...nds, newNode]);
    setSelectedNode(newNode);
    setSelectedEdge(null);
    setRightTab('parameters');
    setDirty(true);
    if (type === 'SCHEDULE') {
      setTriggerPayload(DEFAULT_SCHEDULE_TRIGGER);
    }
  };

  const startBlankPreset = () => {
    blankCanvas();
    const manualId = newNodeId('MANUAL');
    const httpId = newNodeId('HTTP');
    setNodes([
      {
        id: manualId,
        type: 'trigger',
        position: { x: 220, y: 40 },
        data: {
          label: 'Manual Trigger',
          nodeType: 'MANUAL',
          config: {},
          status: 'idle',
        },
      },
      {
        id: httpId,
        type: 'loom',
        position: { x: 200, y: 180 },
        data: {
          label: 'HTTP Request',
          nodeType: 'HTTP',
          config: defaultConfig('HTTP'),
          status: 'idle',
        },
      },
    ]);
    setEdges([
      {
        id: `e-${manualId}-${httpId}`,
        source: manualId,
        target: httpId,
        markerEnd: { type: MarkerType.ArrowClosed },
        data: { condition: '' },
      },
    ]);
    setDirty(true);
    setWorkflowMeta({ name: 'Call an API (blank)' });
    setRightTab('parameters');
    toast('Starter ready — set URL if needed, Save, then Test', 'success');
  };

  const onDragStart = (event, type) => {
    event.dataTransfer.setData('application/reactflow', type);
    event.dataTransfer.effectAllowed = 'move';
  };

  const onDragOver = (event) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
  };

  const onDrop = (event) => {
    event.preventDefault();
    const type = event.dataTransfer.getData('application/reactflow');
    if (!type || !rfInstance) return;
    const position = rfInstance.screenToFlowPosition({
      x: event.clientX,
      y: event.clientY,
    });
    addNode(type, position);
  };

  const deleteStep = (nodeId) => {
    setNodes((nds) => nds.filter((n) => n.id !== nodeId));
    setEdges((eds) => eds.filter((e) => e.source !== nodeId && e.target !== nodeId));
    if (selectedNode?.id === nodeId) setSelectedNode(null);
    setContextMenu(null);
    setDirty(true);
    toast('Step removed', 'success');
  };

  const deleteEdgeById = (edgeId) => {
    setEdges((eds) => eds.filter((e) => e.id !== edgeId));
    if (selectedEdge?.id === edgeId) setSelectedEdge(null);
    setDirty(true);
  };

  useEffect(() => {
    const onKeyDown = (e) => {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') {
        return;
      }
      if (e.key === 'Delete' || e.key === 'Backspace') {
        if (selectedNode) {
          e.preventDefault();
          deleteStep(selectedNode.id);
        } else if (selectedEdge) {
          e.preventDefault();
          deleteEdgeById(selectedEdge.id);
        }
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [selectedNode, selectedEdge]);

  const onConnect = useCallback(
    (params) => {
      setEdges((eds) =>
        addEdge(
          { ...params, markerEnd: { type: MarkerType.ArrowClosed }, data: { condition: '' } },
          eds
        )
      );
      setDirty(true);
    },
    [setEdges]
  );

  const saveWorkflow = async () => {
    const clientErrs = validateFlowClient(nodes, edges);
    if (clientErrs.length) {
      setValidationErrors(clientErrs);
      toast(clientErrs[0], 'error');
      return;
    }
    const dag = flowToLoomDag(nodes, edges);
    try {
      const v = await wfApi.validateDAG(dag);
      if (v && v.valid === false) {
        setValidationErrors([v.error || 'Invalid DAG']);
        toast(v.error || 'Invalid DAG', 'error');
        return;
      }
    } catch (e) {
      toast(e.message, 'error');
      return;
    }
    try {
      if (!activeId) {
        const name = (workflowMeta?.name || '').trim() || 'Untitled Workflow';
        const created = await wfApi.createWorkflow(name, dag);
        const id = parseId(created);
        toast('Workflow saved', 'success');
        await loadWorkflows();
        await openWorkflow(id);

        const start = getStartNode(nodes);
        if (start?.data?.nodeType === 'WEBHOOK') {
          try {
            await wfApi.createWebhook(id);
            const list = await wfApi.listWebhooks(id);
            setWebhooks(list || []);
          } catch {
            /* user can create manually */
          }
        }
        if (start?.data?.nodeType === 'SCHEDULE') {
          const cron = start.data.config?.cronExpression || '0 9 * * *';
          try {
            await wfApi.createSchedule(id, cron);
            const sch = await wfApi.listSchedules(id);
            setSchedules(sch || []);
          } catch {
            /* ignore */
          }
        }
      } else {
        const nextVer = version + 1;
        await wfApi.saveWorkflowVersion(activeId, nextVer, dag);
        setVersion(nextVer);
        toast('Saved version ' + nextVer, 'success');
        setDirty(false);
        bumpSuspendDirty();
      }
      setValidationErrors([]);
    } catch (e) {
      toast(e.message, 'error');
    }
  };

  const runWorkflow = async () => {
    if (!activeId) {
      toast('Save the workflow first', 'error');
      setRightTab('test');
      return;
    }
    let payload;
    try {
      payload = JSON.parse(triggerPayload);
    } catch {
      toast('Invalid JSON in Run tab — fix the sample data', 'error');
      setRightTab('test');
      return;
    }
    try {
      setRunning(true);
      setNodeStatuses({});
      setNodeErrors({});
      setNodeOutputs({});
      const start = getStartNode(nodes);
      const isSchedule = start?.data?.nodeType === 'SCHEDULE';
      setLastTriggerData(null);
      setTriggerInputKind(isSchedule ? 'manual' : null);
      bumpSuspendDirty();
      setNodes((nds) =>
        nds.map((n) => ({
          ...n,
          data: { ...n.data, status: 'idle' },
        }))
      );
      setValidationErrors([]);
      const res = await wfApi.executeWorkflow(
        activeId,
        payload,
        showAdvanced && idempotencyKey ? idempotencyKey : undefined
      );
      toast(isSchedule ? 'Manual test run started…' : 'Running whole workflow…', 'success');
      const ex = await wfApi.listExecutions(activeId);
      setExecutions(ex || []);
      setRightTab('executions');
      const execId = res.execution_id || res.executionId;
      if (execId) await pollExecution(execId);
    } catch (e) {
      toast(e.message, 'error');
    } finally {
      setRunning(false);
    }
  };

  const applyNodeResults = (nodesRes, terminal = false) => {
    bumpSuspendDirty();
    const map = {};
    const errs = {};
    const outs = {};
    (nodesRes || []).forEach((n) => {
      const id = n.nodeId || n.NodeID;
      map[id] = n.status;
      if (n.errorMessage) errs[id] = n.errorMessage;
      if (n.output !== undefined && n.output !== null) {
        outs[id] = typeof n.output === 'string' ? tryParseJson(n.output) : n.output;
      }
    });
    setNodeStatuses(map);
    setNodeErrors(errs);
    setNodeOutputs(outs);
    setNodes((nds) =>
      nds.map((n) => {
        let status = map[n.id] || n.data.status || 'idle';
        // Start nodes are not in worker results — mark success when the run finishes.
        if (terminal && isStartType(n.data?.nodeType) && Object.keys(map).length > 0) {
          status = 'SUCCESS';
        }
        return { ...n, data: { ...n.data, status } };
      })
    );
  };

  const pollExecution = async (execId, opts = {}) => {
    const { silent = false } = opts;
    setActiveExecution(execId);
    return new Promise((resolve) => {
      const poll = async () => {
        try {
          const ex = await execApi.getExecution(execId);
          const status = ex.Status || ex.status;
          const triggerData = parseTriggerFromExecution(ex);
          if (triggerData) {
            setLastTriggerData(triggerData);
            setTriggerInputKind(isScheduledExecution(ex, triggerData) ? 'scheduled' : 'manual');
          }
          const nodesRes = await execApi.getExecutionNodes(execId);
          const list = Array.isArray(nodesRes) ? nodesRes : nodesRes?.value || [];
          const terminal = status === 'COMPLETED' || status === 'FAILED' || status === 'CANCELLED';
          applyNodeResults(list, terminal);
          if (terminal) {
            if (activeId) {
              const listEx = await wfApi.listExecutions(activeId);
              setExecutions(listEx || []);
              if (isScheduledExecution(ex, triggerData)) {
                await refreshSchedules();
              }
            }
            if (!silent) {
              toast(
                status === 'COMPLETED' ? 'Workflow finished — see Results below' : `Workflow ${status}`,
                status === 'COMPLETED' ? 'success' : 'error'
              );
            }
            resolve(status);
            return;
          }
          setTimeout(poll, 1500);
        } catch {
          setTimeout(poll, 2500);
        }
      };
      poll();
    });
  };

  useEffect(() => {
    if (!activeId) return;
    const start = nodes.find((n) => isStartType(n.data?.nodeType));
    if (start?.data?.nodeType !== 'SCHEDULE') return;

    let cancelled = false;
    const tick = async () => {
      try {
        const sch = await wfApi.listSchedules(activeId);
        if (!cancelled) setSchedules(sch || []);

        if (rightTab !== 'executions') return;

        const prevIds = new Set((executionsRef.current || []).map((e) => parseId(e)).filter(Boolean));
        const ex = await wfApi.listExecutions(activeId);
        if (cancelled) return;
        setExecutions(ex || []);
        const newCron = (ex || []).find((e) => {
          const id = parseId(e);
          if (!id || prevIds.has(id)) return false;
          const idemp = e.idempotencyKey || e.IdempotencyKey || '';
          return String(idemp).startsWith('cron-');
        });
        if (newCron) {
          await pollExecution(parseId(newCron), { silent: true });
        }
      } catch {
        /* ignore */
      }
    };
    tick();
    const interval = setInterval(tick, 10000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [activeId, rightTab, nodes]);

  // Refresh next-run label when opening the Schedule node on Configure.
  useEffect(() => {
    if (!activeId || selectedNode?.data?.nodeType !== 'SCHEDULE') return;
    refreshSchedules().catch(() => {});
  }, [activeId, selectedNode?.id, refreshSchedules]);

  function tryParseJson(s) {
    try {
      return JSON.parse(s);
    } catch {
      return s;
    }
  }

  const createWebhook = async () => {
    if (!activeId) return;
    try {
      await wfApi.createWebhook(activeId);
      const list = await wfApi.listWebhooks(activeId);
      setWebhooks(list || []);
      toast('Webhook URL created', 'success');
    } catch (e) {
      toast(e.message, 'error');
    }
  };

  const createSchedule = async (cronExpr) => {
    if (!activeId) {
      toast('Save the workflow first', 'error');
      return;
    }
    try {
      const shouldAutoSave = dirty;
      const created = await wfApi.createSchedule(activeId, cronExpr);
      if (shouldAutoSave) {
        const dag = flowToLoomDag(nodes, edges);
        const nextVer = version + 1;
        await wfApi.saveWorkflowVersion(activeId, nextVer, dag);
        setVersion(nextVer);
        setDirty(false);
        bumpSuspendDirty();
      }
      await refreshSchedules();
      await refreshExecutions();
      setRightTab('executions');
      const next = created.nextRunAt || created.NextRunAt;
      toast(
        next
          ? shouldAutoSave
            ? `Saved workflow + schedule — next run ${formatCronTime(next)}`
            : `Schedule active — next run ${formatCronTime(next)}`
          : 'Schedule active — open Results to watch automatic runs',
        'success'
      );
    } catch (e) {
      toast(e.message, 'error');
    }
  };

  const stopSchedule = async (scheduleId) => {
    if (!activeId || !scheduleId) return;
    try {
      await wfApi.deleteSchedule(activeId, scheduleId);
      await refreshSchedules();
      toast('Schedule stopped — no more automatic runs', 'success');
    } catch (e) {
      toast(e.message, 'error');
    }
  };

  const deleteWorkflow = async () => {
    if (!activeId) return;
    await deleteWorkflowById(activeId, workflowMeta?.name);
  };

  const deleteWorkflowById = async (id, nameHint) => {
    if (!id) return;
    const label = workflowDisplayName(nameHint || 'this workflow');
    if (!confirm(`Delete "${label}" permanently?`)) return;
    try {
      await wfApi.deleteWorkflow(id);
      toast('Workflow deleted', 'success');
      if (activeId === id) {
        blankCanvas();
      }
      await loadWorkflows();
    } catch (e) {
      toast(e.message, 'error');
    }
  };

  const discardUnsaved = () => {
    if (!confirm('Discard this canvas and start fresh?')) return;
    blankCanvas();
    toast('Discarded', 'success');
  };

  const duplicateWorkflow = async () => {
    const clientErrs = validateFlowClient(nodes, edges);
    if (clientErrs.length) {
      toast(clientErrs[0], 'error');
      return;
    }
    const dag = flowToLoomDag(nodes, edges);
    const name = ((workflowMeta?.name || 'Workflow').trim() + ' (copy)').trim();
    try {
      const created = await wfApi.createWorkflow(name, dag);
      await loadWorkflows();
      toast('Duplicated', 'success');
      const id = parseId(created);
      if (id) await openWorkflow(id);
    } catch (e) {
      toast(e.message, 'error');
    }
  };

  const exportJson = () => {
    const dag = flowToLoomDag(nodes, edges);
    const safeName = (workflowMeta?.name || 'workflow').replace(/[^\w.-]+/g, '_');
    const blob = new Blob(
      [JSON.stringify({ name: workflowMeta?.name || 'workflow', dag }, null, 2)],
      { type: 'application/json' }
    );
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `${safeName}.json`;
    a.click();
    URL.revokeObjectURL(a.href);
    toast('Exported', 'success');
  };

  const importJson = (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      try {
        const data = JSON.parse(reader.result);
        const flow = loomDagToFlow(data.dag);
        setNodes(flow.nodes);
        setEdges(flow.edges);
        setWorkflowMeta({ name: data.name || 'Imported' });
        setActiveId(null);
        setDirty(true);
        setSelectedNode(null);
        toast('Imported — click Save to store', 'success');
      } catch {
        toast('Invalid JSON file', 'error');
      }
    };
    reader.readAsText(file);
    e.target.value = '';
  };

  const openTplModal = (t) => {
    const fields = t.config_fields || t.configFields || [];
    const next = {};
    fields.forEach((f) => {
      next[f.key] = f.default || '';
    });
    setTplConfig(next);
    setTplModal(t);
  };

  const confirmTemplate = async () => {
    if (!tplModal) return;
    try {
      const res = await tplApi.createFromTemplate(tplModal.id, tplConfig);
      const reused = res.reused === true;
      toast(reused ? 'Opened existing workflow (not a duplicate)' : 'Template created', 'success');
      const sample = res.sample_trigger || tplModal.sample_trigger || tplModal.sampleTrigger;
      setTplModal(null);
      await loadWorkflows();
      const wfId = res.workflow_id || res.workflowId;
      if (wfId) {
        await openWorkflow(wfId, {
          sampleTrigger: sample && Object.keys(sample).length ? sample : undefined,
          openTest: true,
        });
      }
      if (res.setup_hint || tplModal.setup_hint) {
        toast(res.setup_hint || tplModal.setup_hint, 'info');
      }
    } catch (e) {
      toast(e.message, 'error');
    }
  };

  const updateNodeConfig = (key, value) => {
    if (!selectedNode) return;
    setNodes((nds) =>
      nds.map((n) =>
        n.id === selectedNode.id
          ? { ...n, data: { ...n.data, config: { ...n.data.config, [key]: value } } }
          : n
      )
    );
    setSelectedNode((sn) =>
      sn ? { ...sn, data: { ...sn.data, config: { ...sn.data.config, [key]: value } } } : sn
    );
    setDirty(true);
  };

  const renameNode = (label) => {
    if (!selectedNode) return;
    setNodes((nds) =>
      nds.map((n) => (n.id === selectedNode.id ? { ...n, data: { ...n.data, label } } : n))
    );
    setSelectedNode((sn) => (sn ? { ...sn, data: { ...sn.data, label } } : sn));
    setDirty(true);
  };

  const updateEdgeCondition = (cond) => {
    if (!selectedEdge) return;
    setEdges((eds) =>
      eds.map((e) =>
        e.id === selectedEdge.id
          ? { ...e, label: cond, data: { ...e.data, condition: cond } }
          : e
      )
    );
    setDirty(true);
  };

  const webhookUrl = (wh) => {
    const path = wh.path || wh.Path;
    return `${window.location.origin}/v1/webhooks/${path}`;
  };

  const handleChecklistAction = (action) => {
    if (action === 'build') setSidebarMode('workflows');
    if (action === 'save') saveWorkflow();
    if (action === 'parameters') setRightTab('parameters');
    if (action === 'test') setRightTab('test');
    if (action === 'results') setRightTab('executions');
  };

  const selectedLastRun = selectedNode
    ? {
        status: nodeStatuses[selectedNode.id] || selectedNode.data?.status,
        error: nodeErrors[selectedNode.id],
        output: isStartType(selectedNode.data?.nodeType)
          ? lastTriggerData
          : nodeOutputs[selectedNode.id],
      }
    : null;

  if (loading) {
    return <div className="builder-loading">Loading Loom Builder…</div>;
  }

  const sortedTemplates = [...templates].sort((a, b) => {
    const ar = a.beginner_ready || a.beginnerReady ? 0 : 1;
    const br = b.beginner_ready || b.beginnerReady ? 0 : 1;
    if (ar !== br) return ar - br;
    return (a.name || '').localeCompare(b.name || '');
  });

  const isScheduleWorkflow = getStartNode(nodes)?.data?.nodeType === 'SCHEDULE';

  return (
    <div className="builder-root">
      {apiError && (
        <div className="api-banner">
          <AlertCircle size={16} />
          {apiError}
          <button type="button" onClick={loadWorkflows}>Retry</button>
        </div>
      )}

      <header className="builder-topbar">
        <div className="topbar-left">
          <span className="logo-text">Loom</span>
          <input
            className="workflow-name-input"
            value={workflowMeta?.name || ''}
            placeholder="Workflow name"
            onChange={(e) => {
              setWorkflowMeta((m) => ({ ...m, name: e.target.value }));
              setDirty(true);
            }}
            onBlur={async () => {
              if (activeId && workflowMeta?.name) {
                try {
                  await wfApi.updateWorkflow(activeId, workflowMeta.name);
                } catch (e) {
                  toast(e.message, 'error');
                }
              }
            }}
          />
          {activeId && <span className="version-badge">v{version}</span>}
          {dirty && <span className="dirty-badge">unsaved</span>}
        </div>
        <div className="topbar-actions">
          <button type="button" className="btn-ghost" onClick={exportJson} title="Export">
            <Download size={16} />
          </button>
          <label className="btn-ghost import-label" title="Import">
            <Upload size={16} />
            <input type="file" accept=".json" hidden onChange={importJson} />
          </label>
          <button type="button" className="btn-ghost" onClick={duplicateWorkflow} title="Duplicate">
            <Copy size={16} />
          </button>
          {!activeId && (
            <button type="button" className="btn-ghost btn-discard" onClick={discardUnsaved}>
              Discard
            </button>
          )}
          {activeId && (
            <button type="button" className="btn-ghost btn-delete-workflow" onClick={deleteWorkflow} title="Delete workflow">
              <Trash2 size={16} /> Delete workflow
            </button>
          )}
          <button type="button" className="btn-primary" onClick={saveWorkflow}>
            <Save size={16} /> Save
          </button>
          {!isScheduleWorkflow && (
            <button
              type="button"
              className="btn-accent"
              disabled={running}
              onClick={() => {
                if (!activeId) {
                  toast('Save the workflow first', 'error');
                  return;
                }
                runWorkflow();
              }}
              title="Runs the whole workflow with the JSON in the Run tab"
            >
              <Play size={16} /> {running ? 'Running…' : 'Run'}
            </button>
          )}
        </div>
      </header>

      <div className="builder-body">
        <aside className="builder-sidebar">
          <div className="sidebar-tabs">
            <button
              type="button"
              className={sidebarMode === 'workflows' ? 'active' : ''}
              onClick={() => setSidebarMode('workflows')}
            >
              <List size={14} /> Workflows
            </button>
            <button
              type="button"
              className={sidebarMode === 'templates' ? 'active' : ''}
              onClick={() => setSidebarMode('templates')}
            >
              <LayoutTemplate size={14} /> Gallery
            </button>
          </div>

          {sidebarMode === 'workflows' && (
            <div className="sidebar-list">
              <button type="button" className="sidebar-new" onClick={blankCanvas}>
                <Plus size={14} /> New workflow
              </button>
              {workflows.length === 0 && <div className="empty-state">No workflows yet.</div>}
              {sortWorkflows(workflows).map((w) => {
                const id = parseId(w);
                if (!id) return null;
                const name = workflowDisplayName(w.name || w.Name);
                const when = workflowCreatedLabel(w);
                const tpl = w.templateId || w.template_id;
                return (
                  <div key={id} className="sidebar-workflow-row">
                    <button
                      type="button"
                      className={`sidebar-item ${activeId === id ? 'active' : ''}`}
                      onClick={() => openWorkflow(id)}
                    >
                      <span className="sidebar-item-name">{name}</span>
                      <span className="sidebar-item-meta">
                        {when}
                        {tpl ? ` · ${tpl}` : ''}
                      </span>
                    </button>
                    <button
                      type="button"
                      className="sidebar-item-delete"
                      title="Delete workflow"
                      onClick={() => deleteWorkflowById(id, name)}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                );
              })}
            </div>
          )}

          {sidebarMode === 'templates' && (
            <div className="sidebar-list gallery">
              {sortedTemplates.map((t) => {
                const ready = t.beginner_ready || t.beginnerReady;
                const needsSg = t.needs_sendgrid || t.needsSendGrid;
                return (
                  <div key={t.id} className="gallery-card">
                    <div className="gallery-badges">
                      {FEATURED.has(t.id) && <span className="featured-badge">Featured</span>}
                      {ready && <span className="try-badge">Try now</span>}
                      {needsSg && <span className="sg-badge">Needs SendGrid</span>}
                    </div>
                    <div className="gallery-title">{workflowDisplayName(t.name)}</div>
                    <div className="gallery-desc">{t.description}</div>
                    {t.setup_hint && <div className="gallery-hint">{t.setup_hint}</div>}
                    <button type="button" className="btn-sm" onClick={() => openTplModal(t)}>
                      Use template
                    </button>
                  </div>
                );
              })}
            </div>
          )}

          <div className="node-palette">
            <p className="palette-intro">Drag a node onto the canvas, or click to add.</p>
            {PALETTE_SECTIONS.map((section) => (
              <div key={section.title} className="palette-section">
                <div className="palette-title">{section.title}</div>
                {section.types.map((type) => {
                  const item = NODE_CATALOG[type];
                  const Icon = item.Icon;
                  return (
                    <button
                      key={type}
                      type="button"
                      className="palette-card"
                      draggable
                      onDragStart={(e) => onDragStart(e, type)}
                      onClick={() => addNode(type)}
                    >
                      <div className="palette-card-icon" style={{ color: item.color }}>
                        <Icon size={16} />
                      </div>
                      <div className="palette-card-body">
                        <div className="palette-card-name">{item.name}</div>
                        <div className="palette-card-desc">{item.description}</div>
                      </div>
                    </button>
                  );
                })}
              </div>
            ))}
          </div>
        </aside>

        <main className="builder-canvas" ref={reactFlowWrapper}>
          {nodes.length === 0 && (
            <div className="canvas-empty">
              <h2>Build a workflow</h2>
              <p>
                Drag a node onto the canvas. Connect steps by dragging from the white circle on one node to the next.
              </p>
              <div className="canvas-empty-actions">
                <button type="button" className="btn-accent" onClick={startBlankPreset}>
                  Start blank (Manual + HTTP)
                </button>
                <button type="button" className="btn-primary" onClick={() => setSidebarMode('templates')}>
                  Open Gallery
                </button>
              </div>
            </div>
          )}
          {nodes.length > 0 && nodes.every((n) => isStartType(n.data?.nodeType)) && (
            <div className="canvas-hint">
              <strong>Next:</strong> drag HTTP Request, Send Email, or Transform from the left. Connect with the white circle.
            </div>
          )}
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onInit={setRfInstance}
            onNodesChange={(changes) => {
              onNodesChange(changes);
              if (isDirtySuspended()) return;
              changes.forEach((c) => {
                if (c.type === 'position' && c.dragging === true) userDraggedNodeRef.current = true;
              });
              if (
                changes.some((c) => {
                  if (c.type === 'select' || c.type === 'dimensions') return false;
                  if (c.type === 'position') return c.dragging === false && userDraggedNodeRef.current;
                  return c.type === 'remove' || c.type === 'add' || c.type === 'replace';
                })
              ) {
                setDirty(true);
                userDraggedNodeRef.current = false;
              }
            }}
            onEdgesChange={(changes) => {
              onEdgesChange(changes);
              if (isDirtySuspended()) return;
              if (changes.some((c) => c.type === 'add' || c.type === 'remove' || c.type === 'replace')) {
                setDirty(true);
              }
            }}
            onConnect={onConnect}
            onDrop={onDrop}
            onDragOver={onDragOver}
            nodeTypes={nodeTypes}
            onNodeClick={(_, n) => {
              setSelectedNode(n);
              setSelectedEdge(null);
              setRightTab('parameters');
              setContextMenu(null);
            }}
            onEdgeClick={(_, e) => {
              setSelectedEdge(e);
              setSelectedNode(null);
              setRightTab('parameters');
            }}
            onNodeContextMenu={(e, n) => {
              e.preventDefault();
              setContextMenu({ x: e.clientX, y: e.clientY, nodeId: n.id });
              setSelectedNode(n);
            }}
            onPaneClick={() => {
              setContextMenu(null);
              setSelectedNode(null);
              setSelectedEdge(null);
            }}
            proOptions={{ hideAttribution: true }}
          >
            <Background gap={20} color="rgba(255,255,255,0.04)" />
            <Controls position="bottom-right" showInteractive={false} />
          </ReactFlow>
        </main>

        <aside className="builder-panel">
          <WorkflowReadiness
            nodes={nodes}
            activeId={activeId}
            dirty={dirty}
            webhooks={webhooks}
            schedules={schedules}
            hasRun={(executions || []).length > 0}
            onAction={handleChecklistAction}
          />

          <div className="panel-tabs">
            <button
              type="button"
              className={rightTab === 'parameters' ? 'active' : ''}
              onClick={() => setRightTab('parameters')}
            >
              Configure
            </button>
            <button
              type="button"
              className={rightTab === 'test' ? 'active' : ''}
              onClick={() => setRightTab('test')}
            >
              {isScheduleWorkflow ? 'Test (optional)' : 'Run'}
            </button>
            <button
              type="button"
              className={rightTab === 'executions' ? 'active' : ''}
              onClick={() => setRightTab('executions')}
            >
              Results
            </button>
          </div>

          <div className="panel-body">
            {rightTab === 'parameters' && (
              <p className="tab-purpose">
                <strong>1. Configure</strong> — select a node and set its fields (URL, email, mapping…).
              </p>
            )}
            {rightTab === 'test' && (
              <p className="tab-purpose">
                {isScheduleWorkflow
                  ? (
                    <>
                      <strong>2. Schedule</strong> — runs automatically on cron. Use manual test below only to try it now.
                    </>
                  )
                  : (
                    <>
                      <strong>2. Run</strong> — sample JSON for Manual Trigger, then start the <em>entire</em> workflow (same as top-bar Run).
                    </>
                  )}
              </p>
            )}
            {rightTab === 'executions' && (
              <p className="tab-purpose">
                <strong>3. Results</strong> — JSON each step returned. Click a past run to reload it.
              </p>
            )}

            {rightTab === 'parameters' && selectedNode && (
              <>
                <div className="panel-node-actions">
                  <button type="button" className="btn-sm btn-danger-outline" onClick={() => deleteStep(selectedNode.id)}>
                    <Trash2 size={14} /> Remove
                  </button>
                </div>
                <NodeConfigForm
                  node={selectedNode}
                  onChange={updateNodeConfig}
                  onRename={renameNode}
                  webhooks={webhooks}
                  onCreateWebhook={createWebhook}
                  webhookUrl={webhookUrl}
                  schedules={schedules}
                  onCreateSchedule={createSchedule}
                  onStopSchedule={stopSchedule}
                  formatCronTime={formatCronTime}
                  canCreateWebhook={Boolean(activeId)}
                  allNodes={nodes}
                  allEdges={edges}
                  lastRun={selectedLastRun}
                  toast={toast}
                />
              </>
            )}
            {rightTab === 'parameters' && selectedEdge && (
              <div className="field">
                <label>Branch condition (optional)</label>
                <input
                  value={selectedEdge.data?.condition || ''}
                  onChange={(e) => updateEdgeCondition(e.target.value)}
                  placeholder='trigger.is_vip == "true"'
                />
                <p className="field-hint">Leave empty to always follow this connection.</p>
                <button type="button" className="btn-sm btn-danger-outline" onClick={() => deleteEdgeById(selectedEdge.id)}>
                  Remove connection
                </button>
              </div>
            )}
            {rightTab === 'parameters' && !selectedNode && !selectedEdge && (
              <div className="empty-state">
                Select a node to configure it. Then open <strong>Run</strong> and press Run (or the top-bar Run button).
              </div>
            )}
            {validationErrors.length > 0 && rightTab === 'parameters' && (
              <div className="validation-inline">
                {validationErrors.map((err) => (
                  <div key={err}>{err}</div>
                ))}
              </div>
            )}

            {rightTab === 'test' && (
              <div className="test-panel">
                {!activeId && (
                  <p className="field-hint" style={{ color: '#fbbf24' }}>Save the workflow first, then run.</p>
                )}
                {isScheduleWorkflow && (
                  <p className="field-hint schedule-run-hint">
                    Real runs use the <strong>Schedule</strong> node cron (e.g. every 5 min). They send{' '}
                    <code>{'{"cron_time": "…"}'}</code> — not email/name. Check <strong>Results → Past runs</strong> after the timer fires.
                  </p>
                )}
                <div className="field">
                  <label>{isScheduleWorkflow ? 'Optional manual test JSON' : 'Sample trigger JSON'}</label>
                  <textarea rows={isScheduleWorkflow ? 4 : 10} value={triggerPayload} onChange={(e) => setTriggerPayload(e.target.value)} />
                  <p className="field-hint">
                    {isScheduleWorkflow
                      ? 'Leave as {} for a quick test. HTTP GET steps ignore trigger data anyway.'
                      : (
                        <>
                          Sent into steps as <code>{'{{trigger.*}}'}</code>. Top-bar <strong>Run</strong> uses this JSON.
                        </>
                      )}
                  </p>
                </div>
                {!isScheduleWorkflow && (
                  <button
                    type="button"
                    className="btn-sm"
                    onClick={() => setTriggerPayload(DEFAULT_MANUAL_TRIGGER)}
                  >
                    Load sample
                  </button>
                )}
                <div className="field" style={{ marginTop: '0.75rem' }}>
                  <label>
                    <input type="checkbox" checked={showAdvanced} onChange={(e) => setShowAdvanced(e.target.checked)} />{' '}
                    Advanced
                  </label>
                </div>
                {showAdvanced && (
                  <div className="field">
                    <label>Idempotency key (optional)</label>
                    <input
                      value={idempotencyKey}
                      onChange={(e) => setIdempotencyKey(e.target.value)}
                      placeholder="Leave blank for a new run each time"
                    />
                  </div>
                )}
                <button
                  type="button"
                  className="btn-accent"
                  style={{ width: '100%', marginTop: '0.75rem' }}
                  onClick={runWorkflow}
                  disabled={!activeId || running}
                >
                  <Play size={14} /> {running ? 'Running…' : isScheduleWorkflow ? 'Run manual test' : 'Run whole workflow'}
                </button>
              </div>
            )}

            {rightTab === 'executions' && (
              <div className="executions-panel">
                {isScheduleWorkflow && (
                  <p className="field-hint schedule-run-hint" style={{ marginBottom: '0.75rem' }}>
                    Cron runs appear here automatically — step JSON loads when each run finishes. Use{' '}
                    <strong>Stop schedule</strong> on the Schedule node to turn off the timer.
                  </p>
                )}
                {!isScheduleWorkflow && (
                  <button
                    type="button"
                    className="btn-accent"
                    style={{ width: '100%', marginBottom: '0.75rem' }}
                    onClick={runWorkflow}
                    disabled={!activeId || running}
                  >
                    <Play size={14} /> {running ? 'Running…' : 'Run again'}
                  </button>
                )}

                {(Object.keys(nodeOutputs).length > 0 || lastTriggerData) && (
                  <div className="run-results">
                    <h4 className="results-heading">This run — step outputs</h4>
                    {lastTriggerData && (
                      <div className="result-step">
                        <div className="result-step-head">
                          <span>
                            {triggerInputKind === 'scheduled'
                              ? 'Trigger (scheduled run)'
                              : triggerInputKind === 'manual'
                                ? 'Trigger (manual test)'
                                : 'Trigger (input)'}
                          </span>
                          <span className="status-pill status-completed">START</span>
                        </div>
                        {triggerInputKind === 'scheduled' && lastTriggerData.cron_time && (
                          <p className="field-hint">
                            Scheduled at {formatCronTime(lastTriggerData.cron_time)}
                          </p>
                        )}
                        <pre className="result-json">{JSON.stringify(lastTriggerData, null, 2)}</pre>
                      </div>
                    )}
                    {nodes
                      .filter((n) => !isStartType(n.data?.nodeType))
                      .map((n) => {
                        const st = nodeStatuses[n.id];
                        const err = nodeErrors[n.id];
                        const out = nodeOutputs[n.id];
                        if (!st && out === undefined && !err) return null;
                        return (
                          <div key={n.id} className="result-step">
                            <div className="result-step-head">
                              <span>{n.data.label || n.id}</span>
                              <span className={`status-pill status-${(st || '').toLowerCase()}`}>{st || '—'}</span>
                            </div>
                            {err && <p className="result-error">{err}</p>}
                            {out !== undefined && (
                              <pre className="result-json">{JSON.stringify(out, null, 2)}</pre>
                            )}
                            {!err && out === undefined && st && (
                              <p className="field-hint">No JSON output stored for this step.</p>
                            )}
                          </div>
                        );
                      })}
                  </div>
                )}

                <div className="results-heading-row">
                  <h4 className="results-heading" style={{ marginTop: '1rem' }}>Past runs</h4>
                  <button
                    type="button"
                    className="btn-xs"
                    onClick={() => refreshExecutions().catch((e) => toast(e.message, 'error'))}
                    disabled={!activeId}
                  >
                    Refresh
                  </button>
                </div>
                {executions.length === 0 && (
                  <div className="empty-state">
                    {isScheduleWorkflow && schedules.length > 0
                      ? (
                        <>
                          Cron is active — waiting for the next tick (up to ~1 min). This list refreshes every 15s.
                          Click <strong>Refresh</strong> if nothing appears after a minute.
                        </>
                      )
                      : isScheduleWorkflow
                        ? (
                          <>
                            No runs yet. <strong>Save</strong> the workflow, set cron on the Schedule node, click{' '}
                            <strong>Save schedule</strong>, then wait here (auto-refresh every 15s).
                          </>
                        )
                        : (
                          <>
                            No runs yet. Press <strong>Run</strong> (top bar or Run tab). Then read each step&apos;s JSON above.
                          </>
                        )}
                  </div>
                )}
                {executions.map((ex) => {
                  const id = parseId(ex);
                  const status = ex.status || ex.Status;
                  const idemp = ex.idempotencyKey || ex.IdempotencyKey || '';
                  const scheduled = String(idemp).startsWith('cron-');
                  return (
                    <button
                      key={id}
                      type="button"
                      className={`exec-row ${activeExecution === id ? 'active' : ''}`}
                      onClick={() => {
                        setRightTab('executions');
                        pollExecution(id);
                      }}
                    >
                      <span className={`status-pill status-${(status || '').toLowerCase()}`}>{status}</span>
                      {scheduled && <span className="exec-kind-pill">scheduled</span>}
                      <span className="exec-id">{id}</span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </aside>
      </div>

      {contextMenu && (
        <div
          className="canvas-context-menu"
          style={{ top: contextMenu.y, left: contextMenu.x }}
        >
          <button type="button" onClick={() => deleteStep(contextMenu.nodeId)}>Remove</button>
          <button
            type="button"
            onClick={() => {
              const n = nodes.find((x) => x.id === contextMenu.nodeId);
              if (n) {
                setSelectedNode(n);
                setRightTab('parameters');
              }
              setContextMenu(null);
            }}
          >
            Configure
          </button>
        </div>
      )}

      {tplModal && (
        <div className="modal-overlay">
          <div className="modal">
            <h3>{tplModal.name}</h3>
            <p className="field-hint">{tplModal.setup_hint || tplModal.description}</p>
            {(tplModal.config_fields || tplModal.configFields || []).length === 0 && (
              <p className="field-hint">No setup needed — ready to create and Run.</p>
            )}
            {(tplModal.config_fields || tplModal.configFields || []).map((f) => (
              <div className="field" key={f.key}>
                <label>{f.label || f.key}{f.required ? ' *' : ''}</label>
                <input
                  value={tplConfig[f.key] || ''}
                  onChange={(e) => setTplConfig((c) => ({ ...c, [f.key]: e.target.value }))}
                  placeholder={f.default || ''}
                />
                {f.hint && <p className="field-hint">{f.hint}</p>}
              </div>
            ))}
            <div className="modal-actions">
              <button type="button" onClick={() => setTplModal(null)}>Cancel</button>
              <button type="button" className="btn-primary" onClick={confirmTemplate}>Create</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default function WorkflowBuilder({ toast }) {
  return (
    <ReactFlowProvider>
      <WorkflowBuilderInner toast={toast} />
    </ReactFlowProvider>
  );
}

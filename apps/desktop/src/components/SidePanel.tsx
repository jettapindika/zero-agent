import { CheckCircle2, Circle, CircleDashed, ListTodo, Pencil, Play, ShieldAlert, Square, Wrench, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import type { Task } from '../tasks';
import type { PermissionDecision } from '../permissions';
import { type PermissionRequest } from '../zero-api';
import { DevPanel } from './DevPanel';

// Files moved out to apps/desktop/src/components/FilesList.tsx and now lives
// in the LEFT sidebar above Sessions, so the user can see touched files
// without switching tabs over here. Tabs left: Tasks, Permissions, Run, Dev.
const TAB_LABEL = {
  tasks: 'Tasks',
  perms: 'Permissions',
  run: 'Run',
  dev: 'Dev',
} as const;

export type SidePanelTab = keyof typeof TAB_LABEL;

const STATUS_LABEL: Record<Task['status'], string> = {
  todo: 'To do',
  doing: 'In progress',
  done: 'Done',
};

function taskIcon(status: Task['status']) {
  if (status === 'done') return CheckCircle2;
  if (status === 'doing') return CircleDashed;
  return Circle;
}

export type SidePanelProps = {
  sessionId: string | null;
  sessionModel: string | null;
  sessionAgent: string | null;
  tasks: Task[];
  pending: PermissionRequest[];
  onResolvePermission: (id: string, decision: PermissionDecision) => void | Promise<void>;
  sending: boolean;
  startedAt: number;
  onCancel: () => void;
  onModelClick: () => void;
  onRenameSession: () => void;
  onSendQueueClear: () => void;
  isDev?: boolean;
};

export function SidePanel({
  sessionId,
  sessionModel,
  sessionAgent,
  tasks,
  pending,
  onResolvePermission,
  sending,
  startedAt,
  onCancel,
  onModelClick,
  onRenameSession,
  onSendQueueClear,
  isDev = false,
}: SidePanelProps) {
  const [tab, setTab] = useState<SidePanelTab>('tasks');
  const [now, setNow] = useState<number>(Date.now());

  // Auto-switch to the permissions tab when a request lands. Polling lives
  // in App.tsx via usePendingPermissions; this side effect just reacts to
  // the array growing.
  useEffect(() => {
    if (pending.length > 0) setTab('perms');
  }, [pending.length]);

  // Tick the elapsed timer while sending.
  useEffect(() => {
    if (!sending) return;
    const id = window.setInterval(() => setNow(Date.now()), 500);
    return () => window.clearInterval(id);
  }, [sending]);

  function handlePermission(p: PermissionRequest, decision: PermissionDecision) {
    void onResolvePermission(p.id, decision);
  }

  const taskGroups: Record<Task['status'], Task[]> = { doing: [], todo: [], done: [] };
  for (const task of tasks) taskGroups[task.status].push(task);

  const elapsedSec = sending ? Math.max(0, Math.floor((now - startedAt) / 1000)) : 0;
  const elapsedLabel = elapsedSec >= 60
    ? `${Math.floor(elapsedSec / 60)}:${(elapsedSec % 60).toString().padStart(2, '0')}`
    : `${elapsedSec}s`;

  return (
    <aside className="side-panel" aria-label="Side panel">
      <nav className="side-tabs" role="tablist">
        {(Object.keys(TAB_LABEL) as SidePanelTab[])
          .filter((key) => key !== 'dev' || isDev)
          .map((key) => {
            const badge = key === 'tasks' ? tasks.length
              : key === 'perms' ? pending.length
              : 0;
            return (
              <button
                aria-selected={tab === key}
                className={tab === key ? 'side-tab active' : 'side-tab'}
                key={key}
                onClick={() => setTab(key)}
                role="tab"
                type="button"
              >
                {key === 'dev' ? <Wrench size={12} /> : null}
                {TAB_LABEL[key]}
                {badge > 0 ? <span className="side-tab-badge">{badge}</span> : null}
              </button>
            );
          })}
      </nav>

      <div className="side-body">
        {tab === 'tasks' ? (
          tasks.length === 0 ? (
            <p className="side-empty">
              <ListTodo size={18} />
              No tasks yet. Ask Zero to use task markers in its replies.
            </p>
          ) : (
            (['doing', 'todo', 'done'] as Task['status'][]).map((status) => {
              const items = taskGroups[status];
              if (items.length === 0) return null;
              return (
                <section className={`task-group ${status}`} key={status}>
                  <p className="task-group-label">{STATUS_LABEL[status]}</p>
                  <ul>
                    {items.map((task) => {
                      const Icon = taskIcon(task.status);
                      return (
                        <li className={`task-row ${task.status}`} key={task.id}>
                          <Icon size={14} />
                          <div className="task-text">
                            <p className="task-title">{task.title}</p>
                            {task.notes ? <p className="task-notes">{task.notes}</p> : null}
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                </section>
              );
            })
          )
        ) : null}

        {tab === 'perms' ? (
          pending.length === 0 ? (
            <p className="side-empty">
              <ShieldAlert size={18} />
              No pending permission requests.
            </p>
          ) : (
            <ul className="perm-list">
              {pending.map((p) => (
                <li className="perm-row" key={p.id}>
                  <p className="perm-tool">{p.toolName}</p>
                  <pre className="perm-args">{JSON.stringify(p.args, null, 2)}</pre>
                  <div className="perm-actions">
                    <button onClick={() => handlePermission(p, 'allow_once')} type="button">Allow once</button>
                    <button onClick={() => handlePermission(p, 'always_allow')} type="button">Always allow</button>
                    <button className="danger" onClick={() => handlePermission(p, 'deny')} type="button">Deny</button>
                  </div>
                </li>
              ))}
            </ul>
          )
        ) : null}

        {tab === 'run' ? (
          <div className="run-panel">
            <p className="side-eyebrow">Status</p>
            <p className="run-status">
              {sending ? <Play size={14} /> : <Square size={14} />}
              {sending ? `Running · ${elapsedLabel}` : 'Idle'}
            </p>
            {sending ? (
              <button className="run-cancel" onClick={onCancel} type="button">
                <X size={14} /> Cancel run (Esc Esc)
              </button>
            ) : null}
            <p className="side-eyebrow">Active session</p>
            <button className="run-meta" onClick={onModelClick} type="button">
              <span>{sessionModel ?? '—'}</span>
              <small>{sessionAgent ?? 'agent'} · click to change</small>
            </button>
            <button className="run-meta" onClick={onRenameSession} type="button">
              <span><Pencil size={12} /> Rename session</span>
              <small>Override the auto-title</small>
            </button>
            <p className="side-eyebrow">Queue</p>
            <button className="run-meta" onClick={onSendQueueClear} type="button">
              <span>Clear pending prompts</span>
              <small>Same as /clear</small>
            </button>
          </div>
        ) : null}

        {tab === 'dev' && isDev ? <DevPanel /> : null}
      </div>

      <footer className="side-foot">
        <span>{sessionAgent ?? 'agent'}</span>
        <span className="side-foot-model">{sessionModel ?? 'no model'}</span>
      </footer>
    </aside>
  );
}

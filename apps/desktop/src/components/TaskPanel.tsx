import { CheckCircle2, Circle, CircleDashed } from 'lucide-react';
import type { Task } from '../tasks';

const STATUS_LABEL: Record<Task['status'], string> = {
  todo: 'To do',
  doing: 'In progress',
  done: 'Done',
};

function iconFor(status: Task['status']) {
  if (status === 'done') return CheckCircle2;
  if (status === 'doing') return CircleDashed;
  return Circle;
}

export type TaskPanelProps = {
  tasks: Task[];
};

export function TaskPanel({ tasks }: TaskPanelProps) {
  const groups: Record<Task['status'], Task[]> = { doing: [], todo: [], done: [] };
  for (const task of tasks) groups[task.status].push(task);

  const total = tasks.length;
  const done = groups.done.length;

  return (
    <aside className="task-panel" aria-label="Task list">
      <header className="task-header">
        <div>
          <p className="eyebrow">Tasks</p>
          <h2>{total === 0 ? 'No tasks yet' : `${done}/${total} complete`}</h2>
        </div>
      </header>
      <div className="task-body">
        {(['doing', 'todo', 'done'] as Task['status'][]).map((status) => {
          const items = groups[status];
          if (items.length === 0) return null;
          return (
            <section className={`task-group ${status}`} key={status}>
              <p className="task-group-label">{STATUS_LABEL[status]}</p>
              <ul>
                {items.map((task) => {
                  const Icon = iconFor(task.status);
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
        })}
        {total === 0 ? (
          <p className="task-empty">
            Ask Zero to use task markers in its replies, e.g.<br />
            <code>[task] Build login flow</code><br />
            <code>[task-doing] Build login flow</code><br />
            <code>[task-done] Build login flow</code>
          </p>
        ) : null}
      </div>
    </aside>
  );
}

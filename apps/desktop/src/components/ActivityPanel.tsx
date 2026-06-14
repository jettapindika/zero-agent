import { AlertTriangle, Bot, FileText, Search, Terminal } from 'lucide-react';
import type { ActivityItem } from '../activity';

const KIND_ICON: Record<ActivityItem['kind'], typeof Bot> = {
  thinking: Bot,
  tool_call: Terminal,
  tool_result: FileText,
  text: FileText,
  status: Search,
};

function iconFor(item: ActivityItem) {
  if (item.status === 'error') return AlertTriangle;
  if (item.kind === 'tool_call' && /grep|glob|search/i.test(item.label)) return Search;
  if (item.kind === 'tool_call' && /bash/i.test(item.label)) return Terminal;
  if (item.kind === 'tool_call' && /(read|write|edit|ls|walk)/i.test(item.label)) return FileText;
  return KIND_ICON[item.kind] ?? Bot;
}

export type ActivityPanelProps = {
  items: ActivityItem[];
  startedAt: number;
};

export function ActivityPanel({ items, startedAt }: ActivityPanelProps) {
  const elapsed = Math.max(0, Math.floor((Date.now() - startedAt) / 1000));
  const minutes = Math.floor(elapsed / 60);
  const seconds = elapsed % 60;
  const elapsedLabel = minutes > 0 ? `${minutes}:${seconds.toString().padStart(2, '0')}` : `${seconds}s`;

  const pendingCount = items.filter((i) => i.status === 'pending').length;
  const summary = pendingCount > 0 ? `${pendingCount} action${pendingCount === 1 ? '' : 's'} in flight · ${elapsedLabel}` : `Working · ${elapsedLabel}`;

  return (
    <section className="activity-panel" role="status" aria-live="polite">
      <header className="activity-header">
        <div className="orbital-loader" aria-hidden="true">
          <span />
          <span />
          <span />
        </div>
        <div>
          <p>Zero is thinking</p>
          <small>{summary}</small>
        </div>
      </header>
      <ol className="activity-body">
        {items.length === 0 ? <li className="activity-empty">Streaming response… activity will appear here when Zero uses tools or thinks aloud.</li> : null}
        {items.map((item) => {
          const Icon = iconFor(item);
          return (
            <li className={`activity-row ${item.status}`} key={item.id}>
              <Icon size={14} />
              <div className="activity-text">
                <p className="activity-label">{item.label}</p>
                {item.detail ? <p className="activity-detail">{item.detail}</p> : null}
              </div>
              <span className={`activity-status ${item.status}`}>
                {item.status === 'pending' ? '…' : item.status === 'error' ? '!' : '✓'}
              </span>
            </li>
          );
        })}
      </ol>
    </section>
  );
}

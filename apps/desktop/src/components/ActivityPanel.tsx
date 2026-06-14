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
};

export function ActivityPanel({ items }: ActivityPanelProps) {
  if (items.length === 0) return null;

  return (
    <section className="activity-panel" role="status" aria-live="polite">
      <ol className="activity-body">
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

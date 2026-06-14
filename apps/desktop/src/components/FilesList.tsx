// FilesList renders the list of files Zero has read/written/edited in the
// current session. Lives in the LEFT sidebar above the Sessions list so the
// user always sees what's been touched without switching tabs in the right
// SidePanel.

import { FileEdit, FileText } from 'lucide-react';
import type { TouchedFile } from '../files';

type Props = {
  files: TouchedFile[];
};

function fileIcon(ops: TouchedFile['ops']) {
  if (ops.includes('write') || ops.includes('edit')) return FileEdit;
  return FileText;
}

// classifyOps picks the most informative status badge to show on the right
// of each row. Order matters: writes/edits dominate read-style ops because
// they're the actionable change, not just observation.
function classifyOps(ops: TouchedFile['ops']): { label: string; tone: 'modified' | 'read' } {
  if (ops.includes('write')) return { label: 'new', tone: 'modified' };
  if (ops.includes('edit')) return { label: 'modified', tone: 'modified' };
  return { label: 'read', tone: 'read' };
}

// shortenPath keeps the basename + the immediately enclosing directory so
// `services/core/internal/auth/oauth.go` becomes `auth/oauth.go`. Long
// paths still render in full as the title attribute on hover.
function shortenPath(path: string): string {
  const parts = path.split('/').filter(Boolean);
  if (parts.length <= 2) return path;
  return parts.slice(-2).join('/');
}

export function FilesList({ files }: Props) {
  if (files.length === 0) {
    return (
      <section className="files-panel">
        <p className="files-panel-label">Files</p>
        <p className="files-panel-empty">No files touched yet.</p>
      </section>
    );
  }
  return (
    <section className="files-panel">
      <p className="files-panel-label">
        Files <span className="files-panel-count">{files.length}</span>
      </p>
      <ul className="files-panel-list">
        {files.map((file) => {
          const Icon = fileIcon(file.ops);
          const status = classifyOps(file.ops);
          return (
            <li className="files-panel-row" key={file.path} title={file.path}>
              <Icon size={12} />
              <span className="files-panel-name">{shortenPath(file.path)}</span>
              <span className={`files-panel-status ${status.tone}`}>{status.label}</span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

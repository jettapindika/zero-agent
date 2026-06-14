// FilesList renders the list of files Zero has read/written/edited in the
// current session, plus any attached files. Lives in the LEFT sidebar above
// the Sessions list so the user always sees what's been touched without
// switching tabs in the right SidePanel.

import { File as FileIcon, FileEdit, FileImage, FileSpreadsheet, FileText, Paperclip } from 'lucide-react';
import type { TouchedFile } from '../files';
import type { DroppedKind } from '../dropped-files';

export type AttachedFile = {
  name: string;
  kind: DroppedKind;
  status: 'uploading' | 'done' | 'error';
};

type Props = {
  files: TouchedFile[];
  attachments?: AttachedFile[];
};

function fileIcon(ops: TouchedFile['ops']) {
  if (ops.includes('write') || ops.includes('edit')) return FileEdit;
  return FileText;
}

function attachmentIcon(kind: DroppedKind) {
  if (kind === 'IMG') return FileImage;
  if (kind === 'PDF' || kind === 'XLS' || kind === 'DOC') return FileSpreadsheet;
  return FileIcon;
}

function attachmentStatusLabel(status: AttachedFile['status']): string {
  if (status === 'uploading') return '●';
  if (status === 'done') return '✓';
  return '✗';
}

function attachmentStatusClass(status: AttachedFile['status']): string {
  if (status === 'uploading') return 'files-panel-status uploading';
  if (status === 'done') return 'files-panel-status done';
  return 'files-panel-status error';
}

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

export function FilesList({ files, attachments = [] }: Props) {
  const hasContent = files.length > 0 || attachments.length > 0;
  
  if (!hasContent) {
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
        Files <span className="files-panel-count">{files.length + attachments.length}</span>
      </p>
      <ul className="files-panel-list">
        {attachments.map((att, idx) => {
          const Icon = attachmentIcon(att.kind);
          return (
            <li className="files-panel-row" key={`att-${idx}-${att.name}`} title={att.name}>
              <Paperclip size={10} style={{ marginRight: '2px' }} />
              <Icon size={12} />
              <span className="files-panel-name">{att.name}</span>
              <span className={attachmentStatusClass(att.status)}>
                {attachmentStatusLabel(att.status)}
              </span>
            </li>
          );
        })}
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

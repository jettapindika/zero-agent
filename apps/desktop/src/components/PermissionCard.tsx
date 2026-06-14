// PermissionCard surfaces a pending permission request inline in the chat
// stream so the user can't miss it. The same data drives the SidePanel
// "Permissions" tab; resolving from either place clears the card here.

import { ShieldAlert } from 'lucide-react';
import type { PermissionDecision } from '../permissions';
import type { PermissionRequest } from '../zero-api';

type Props = {
  request: PermissionRequest;
  onResolve: (decision: PermissionDecision) => void | Promise<void>;
};

// describe formats the args object into one short sentence — `bash · pwd`,
// `write · /tmp/x.txt`, etc. Falls back to the JSON dump when no familiar
// arg key is present.
function describe(req: PermissionRequest): string {
  const args = req.args ?? {};
  const path = (args.path as string) || (args.command as string) || (args.pattern as string) || '';
  if (path) return `${req.toolName} · ${path}`;
  try {
    return `${req.toolName} · ${JSON.stringify(args)}`;
  } catch {
    return req.toolName;
  }
}

export function PermissionCard({ request, onResolve }: Props) {
  return (
    <article className="permission-card" role="alert" aria-live="assertive">
      <ShieldAlert size={18} />
      <div className="permission-card-body">
        <p className="permission-card-title">Permission needed</p>
        <p className="permission-card-detail">{describe(request)}</p>
        <div className="permission-card-actions">
          <button
            className="permission-action approve"
            onClick={() => void onResolve('allow_once')}
            type="button"
          >
            Allow once
          </button>
          <button
            className="permission-action approve-always"
            onClick={() => void onResolve('always_allow')}
            type="button"
          >
            Always allow
          </button>
          <button
            className="permission-action deny"
            onClick={() => void onResolve('deny')}
            type="button"
          >
            Deny
          </button>
        </div>
      </div>
    </article>
  );
}

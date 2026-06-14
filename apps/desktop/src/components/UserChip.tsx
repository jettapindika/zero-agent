// Avatar + display name in the topbar. Clicking opens a small dropdown with
// "Signed in as <email>", role badge, and a Sign out action.

import { useEffect, useRef, useState } from 'react';
import type { AuthUser } from '../zero-api';

type Props = {
  user: AuthUser;
  isDev: boolean;
  onSignOut: () => Promise<void> | void;
};

export function UserChip({ user, isDev, onSignOut }: Props) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement | null>(null);

  // Click-outside closes the dropdown without trapping focus.
  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', onDoc);
    return () => document.removeEventListener('mousedown', onDoc);
  }, [open]);

  const initials = (user.displayName || user.email || '?')
    .split(/\s+/)
    .map((p: string) => p[0])
    .filter(Boolean)
    .slice(0, 2)
    .join('')
    .toUpperCase();

  return (
    <div className="user-chip" ref={ref}>
      <button
        aria-expanded={open}
        aria-haspopup="menu"
        className="user-chip-trigger"
        onClick={() => setOpen((v) => !v)}
        type="button"
      >
        {user.avatarUrl ? (
          <img alt="" className="user-chip-avatar" referrerPolicy="no-referrer" src={user.avatarUrl} />
        ) : (
          <span aria-hidden="true" className="user-chip-avatar fallback">{initials}</span>
        )}
        <span className="user-chip-name">{user.displayName || user.email}</span>
        {isDev ? <span className="user-chip-badge dev">DEV</span> : null}
      </button>

      {open ? (
        <div className="user-chip-menu" role="menu">
          <p className="user-chip-menu-eyebrow">Signed in as</p>
          <p className="user-chip-menu-email">{user.email}</p>
          <p className="user-chip-menu-meta">role: <code>{user.role}</code></p>
          <hr />
          <button
            className="user-chip-menu-action"
            onClick={async () => {
              setOpen(false);
              await onSignOut();
            }}
            role="menuitem"
            type="button"
          >
            Sign out
          </button>
        </div>
      ) : null}
    </div>
  );
}

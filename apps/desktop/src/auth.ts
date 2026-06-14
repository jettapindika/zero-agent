// Authentication hook for the Tauri desktop app.
//
// The Go daemon owns the session cookie; the desktop simply asks /auth/me on
// startup, on window-focus, and whenever an SSE bus event announces a sign-in
// or sign-out. The cookie travels automatically because zero-api.ts sends
// credentials: 'include' on every request.

import { useCallback, useEffect, useRef, useState } from 'react';
import {
  AuthRequiredError,
  authLogout,
  authMe,
  AUTH_START_URL,
  setSessionToken,
  type AuthMeResponse,
  type AuthUser,
} from './zero-api';
import { desktop } from './tauri';

const API_BASE = 'http://127.0.0.1:8910';

// One-time per-launch claim the desktop binds to its sign-in attempt. The
// daemon echoes it back on the auth.signed_in bus event so we know the
// signedToken that follows belongs to OUR /signin click, not someone else's.
function newClaim(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

export type AuthState =
  | { status: 'loading'; user: null; isDev: false }
  | { status: 'signed_out'; user: null; isDev: false; reason?: string }
  | { status: 'signed_in'; user: AuthUser; isDev: boolean; sessionId: string };

export type UseAuth = {
  state: AuthState;
  refresh: () => Promise<void>;
  signIn: () => Promise<void>;
  signOut: () => Promise<void>;
};

export function useCurrentUser(): UseAuth {
  const [state, setState] = useState<AuthState>({ status: 'loading', user: null, isDev: false });
  // Latest in-flight claim. Lives in a ref so the SSE handler always sees the
  // current value even if a re-render hasn't run yet.
  const pendingClaim = useRef<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const me: AuthMeResponse = await authMe();
      setState({
        status: 'signed_in',
        user: me.user,
        isDev: me.isDev,
        sessionId: me.sessionId,
      });
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        setState({ status: 'signed_out', user: null, isDev: false });
        return;
      }
      // Daemon offline / network blip — treat as signed_out so the LoginView
      // shows a useful message rather than spinning forever.
      setState({
        status: 'signed_out',
        user: null,
        isDev: false,
        reason: (err as Error).message,
      });
    }
  }, []);

  const signIn = useCallback(async () => {
    // Mint a one-shot claim and pass it on the start URL. The daemon binds
    // the claim to this OAuth flow and echoes it back via the bus event so we
    // know which signed_in is ours. Without this hook the Tauri webview
    // would never see the cookie that the system browser collected.
    const claim = newClaim();
    pendingClaim.current = claim;
    const url = `${AUTH_START_URL}?claim=${encodeURIComponent(claim)}`;
    try {
      await desktop.openExternalUrl(url);
    } catch {
      window.open(url, '_blank');
    }
  }, []);

  const signOut = useCallback(async () => {
    try {
      await authLogout();
    } finally {
      setSessionToken(null);
      setState({ status: 'signed_out', user: null, isDev: false });
    }
  }, []);

  // Initial fetch + window-focus refetch — covers the "user signed in via
  // browser then alt-tabbed back" path on machines where SSE may be slow.
  useEffect(() => {
    void refresh();
    const onFocus = () => {
      if (state.status !== 'loading') void refresh();
    };
    window.addEventListener('focus', onFocus);
    return () => window.removeEventListener('focus', onFocus);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refresh]);

  // Subscribe to auth bus events so the UI updates the moment the daemon
  // accepts the OAuth callback. The SSE handler is the only path that
  // delivers the bearer token to a Tauri webview — fetch's cookie jar can't
  // see what the system browser saved.
  useEffect(() => {
    const source = new EventSource(`${API_BASE}/events`);
    const onSignedIn = (ev: MessageEvent) => {
      let payload: { claim?: string; sessionToken?: string } = {};
      try {
        const parsed = JSON.parse(ev.data);
        payload = parsed?.payload ?? {};
      } catch {
        /* malformed event; ignore */
      }
      // Drop events we didn't ask for. If no claim was bound (older daemon)
      // we still refresh — same as previous behavior.
      if (payload.claim && payload.claim !== pendingClaim.current) {
        return;
      }
      pendingClaim.current = null;
      if (payload.sessionToken) {
        setSessionToken(payload.sessionToken);
      }
      void refresh();
    };
    const onSignedOut = () => {
      setSessionToken(null);
      setState({ status: 'signed_out', user: null, isDev: false });
    };
    source.addEventListener('auth.signed_in', onSignedIn as EventListener);
    source.addEventListener('auth.signed_out', onSignedOut as EventListener);
    return () => {
      source.removeEventListener('auth.signed_in', onSignedIn as EventListener);
      source.removeEventListener('auth.signed_out', onSignedOut as EventListener);
      source.close();
    };
  }, [refresh]);

  return { state, refresh, signIn, signOut };
}

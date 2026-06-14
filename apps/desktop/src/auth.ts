// Authentication hook for the Tauri desktop app.
//
// The Go daemon owns the session cookie; the desktop simply asks /auth/me on
// startup, on window-focus, and whenever an SSE bus event announces a sign-in
// or sign-out. The cookie travels automatically because zero-api.ts sends
// credentials: 'include' on every request.

import { useCallback, useEffect, useState } from 'react';
import {
  AuthRequiredError,
  authLogout,
  authMe,
  AUTH_START_URL,
  type AuthMeResponse,
  type AuthUser,
} from './zero-api';
import { desktop } from './tauri';

const API_BASE = 'http://127.0.0.1:8910';

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
    // Open the system browser — the daemon's /auth/google/start sets PKCE +
    // state + redirects to Google. After consent, /auth/google/callback writes
    // the cookie and broadcasts auth.signed_in on the bus.
    try {
      await desktop.openExternalUrl(AUTH_START_URL);
    } catch {
      // Fall back to window.open in the rare case the Tauri shell command is
      // unavailable; this still works inside dev (vite preview).
      window.open(AUTH_START_URL, '_blank');
    }
  }, []);

  const signOut = useCallback(async () => {
    try {
      await authLogout();
    } finally {
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
  // accepts the OAuth callback.
  useEffect(() => {
    const source = new EventSource(`${API_BASE}/events`);
    const onSignedIn = () => {
      void refresh();
    };
    const onSignedOut = () => {
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

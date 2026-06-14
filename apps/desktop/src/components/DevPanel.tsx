// DevPanel renders inside the SidePanel under the "Dev" tab. Visible only
// when /auth/me reports isDev: true. Backend endpoints used here are gated by
// auth.RequireDev so non-dev callers see 403 even if they hit them directly.

import { useEffect, useState } from 'react';

const API_BASE = 'http://127.0.0.1:8910';

type RuntimeInfo = {
  goVersion?: string;
  goroutines?: number;
  numCPU?: number;
  goos?: string;
  goarch?: string;
  uptimeSeconds?: number;
  module?: string;
  version?: string;
};

const FEATURE_FLAGS = [
  { key: 'experimentalStreaming', label: 'Experimental token streaming' },
  { key: 'verboseLogging', label: 'Verbose console logging' },
  { key: 'taskMarkersEnabled', label: 'Render [task] markers' },
  { key: 'slashPreviewAlwaysVisible', label: 'Always show slash preview' },
] as const;

function readFlag(key: string): boolean {
  try {
    return window.localStorage.getItem(`zero.flags.${key}`) === '1';
  } catch {
    return false;
  }
}

function writeFlag(key: string, on: boolean) {
  try {
    if (on) window.localStorage.setItem(`zero.flags.${key}`, '1');
    else window.localStorage.removeItem(`zero.flags.${key}`);
  } catch {
    /* localStorage unavailable; ignore */
  }
}

export function DevPanel() {
  const [runtime, setRuntime] = useState<RuntimeInfo | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [reloadMsg, setReloadMsg] = useState<string | null>(null);
  const [flags, setFlags] = useState<Record<string, boolean>>(() => {
    const init: Record<string, boolean> = {};
    for (const f of FEATURE_FLAGS) init[f.key] = readFlag(f.key);
    return init;
  });

  // Pull runtime info on mount; refresh every 5s while panel is mounted so
  // the goroutine count reflects live state.
  useEffect(() => {
    let cancelled = false;
    async function fetchRuntime() {
      try {
        const res = await fetch(`${API_BASE}/dev/runtime`, { credentials: 'include' });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const json = (await res.json()) as RuntimeInfo;
        if (!cancelled) {
          setRuntime(json);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) setError((err as Error).message);
      }
    }
    void fetchRuntime();
    const id = window.setInterval(fetchRuntime, 5000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, []);

  function toggleFlag(key: string) {
    setFlags((current) => {
      const next = !current[key];
      writeFlag(key, next);
      return { ...current, [key]: next };
    });
  }

  async function reloadSkills() {
    try {
      const res = await fetch(`${API_BASE}/dev/skills/reload`, {
        method: 'POST',
        credentials: 'include',
      });
      if (!res.ok && res.status !== 204) throw new Error(`HTTP ${res.status}`);
      setReloadMsg('Reload signal sent.');
      window.setTimeout(() => setReloadMsg(null), 2500);
    } catch (err) {
      setReloadMsg(`Failed: ${(err as Error).message}`);
    }
  }

  const uptime = formatUptime(runtime?.uptimeSeconds ?? 0);

  return (
    <div className="dev-panel">
      <div className="dev-banner">DEV MODE</div>

      <p className="side-eyebrow">Runtime</p>
      {error ? <p className="dev-error">/dev/runtime: {error}</p> : null}
      <ul className="dev-runtime">
        <li><span>Go</span><code>{runtime?.goVersion ?? '—'}</code></li>
        <li><span>OS / arch</span><code>{runtime?.goos ?? '—'} / {runtime?.goarch ?? '—'}</code></li>
        <li><span>Goroutines</span><code>{runtime?.goroutines ?? '—'}</code></li>
        <li><span>CPUs</span><code>{runtime?.numCPU ?? '—'}</code></li>
        <li><span>Uptime</span><code>{uptime}</code></li>
        {runtime?.version ? <li><span>Build</span><code>{runtime.version}</code></li> : null}
      </ul>

      <p className="side-eyebrow">Feature flags</p>
      <ul className="dev-flags">
        {FEATURE_FLAGS.map((f) => (
          <li key={f.key}>
            <label>
              <input
                checked={flags[f.key]}
                onChange={() => toggleFlag(f.key)}
                type="checkbox"
              />
              <span>{f.label}</span>
            </label>
          </li>
        ))}
      </ul>

      <p className="side-eyebrow">Maintenance</p>
      <button className="run-meta" onClick={reloadSkills} type="button">
        <span>Reload skills</span>
        <small>Re-scan SKILL.md files on next prompt</small>
      </button>
      {reloadMsg ? <p className="dev-msg">{reloadMsg}</p> : null}
    </div>
  );
}

function formatUptime(seconds: number): string {
  if (!seconds || seconds < 0) return '—';
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

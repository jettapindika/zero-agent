import { useEffect, useState } from 'react';
import { Copy, Loader2, Square, Users } from 'lucide-react';
import { clearShareConfig, getIdentity, revokeRoom, roomEventsUrl, type ShareConfig } from '../collab';

type Props = {
  config: ShareConfig;
  onStopped: () => void;
};

export function CollabBar({ config, onStopped }: Props) {
  const [guestCount, setGuestCount] = useState(0);
  const [copied, setCopied] = useState<'code' | 'link' | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const url = roomEventsUrl(config.sessionId);
    const es = new EventSource(url, { withCredentials: true });
    const onJoined = () => setGuestCount((c) => c + 1);
    const onLeft = () => setGuestCount((c) => Math.max(0, c - 1));
    es.addEventListener('participant.joined', onJoined);
    es.addEventListener('participant.left', onLeft);
    es.onerror = () => {};
    return () => {
      es.removeEventListener('participant.joined', onJoined);
      es.removeEventListener('participant.left', onLeft);
      es.close();
    };
  }, [config.sessionId]);

  function copy(value: string, kind: 'code' | 'link') {
    navigator.clipboard
      .writeText(value)
      .then(() => {
        setCopied(kind);
        setTimeout(() => setCopied(null), 1500);
      })
      .catch(() => setError('Could not copy to clipboard'));
  }

  async function stop() {
    setBusy(true);
    setError(null);
    try {
      const identity = await getIdentity();
      await revokeRoom(identity, config.sessionId);
      clearShareConfig();
      onStopped();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setBusy(false);
    }
  }

  const folderName = config.folderPath.split('/').filter(Boolean).pop() || config.folderPath;

  return (
    <div className="collab-bar" role="status" aria-label="Active share session">
      <span className="collab-dot" aria-hidden="true" />
      <span className="collab-text">
        Sharing <span className="collab-folder">{folderName}</span>
      </span>
      <span className="collab-sep">·</span>
      <span className="collab-meta">
        <Users size={12} /> {guestCount} guest{guestCount === 1 ? '' : 's'}
      </span>
      <span className="collab-sep">·</span>
      <span className="collab-meta">
        code:&nbsp;
        <code className="collab-code">{config.inviteCode}</code>
        <button
          type="button"
          className="collab-icon-btn"
          aria-label="Copy invite code"
          onClick={() => copy(config.inviteCode, 'code')}
        >
          <Copy size={12} />
        </button>
        {copied === 'code' ? <small className="collab-copied">copied</small> : null}
      </span>
      <span className="collab-sep">·</span>
      <button type="button" className="collab-link-btn" onClick={() => copy(config.inviteUrl, 'link')}>
        copy link {copied === 'link' ? <small className="collab-copied">copied</small> : null}
      </button>
      <span className="collab-sep">·</span>
      <span className="collab-meta">
        token:&nbsp;<span className={`collab-token-${config.tokenMode}`}>{config.tokenMode}</span>
        {config.tokenMode === 'host' && config.rateLimit ? (
          <>&nbsp;· {config.rateLimit} req/min</>
        ) : null}
      </span>

      {error ? <span className="collab-error">{error}</span> : null}

      <button type="button" className="collab-stop" onClick={stop} disabled={busy}>
        {busy ? <Loader2 size={12} className="spin" /> : <Square size={12} />}
        {busy ? 'Stopping…' : 'Stop sharing'}
      </button>
    </div>
  );
}

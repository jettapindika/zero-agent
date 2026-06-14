import { AlertTriangle, Loader2 } from 'lucide-react';
import { useState } from 'react';
import { getIdentity, interruptPrompt } from '../collab';

type Props = {
  actorNickname: string;
  roomId: string;
  sessionId: string;
  onInterrupted?: () => void;
};

export function PromptInterruptWarning({ actorNickname, roomId, sessionId, onInterrupted }: Props) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleJoin() {
    setBusy(true);
    setError(null);
    try {
      const identity = await getIdentity();
      await interruptPrompt(identity, roomId, sessionId);
      onInterrupted?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="prompt-interrupt-warning" role="alert">
      <AlertTriangle size={14} className="prompt-interrupt-icon" />
      <span className="prompt-interrupt-text">
        <strong>{actorNickname}</strong> is prompting. Sending will interrupt their session.
      </span>
      <button
        type="button"
        className="prompt-interrupt-btn"
        onClick={handleJoin}
        disabled={busy}
      >
        {busy ? <Loader2 size={12} className="spin" /> : null}
        {busy ? 'Joining…' : 'Join Session'}
      </button>
      {error ? <span className="prompt-interrupt-error">{error}</span> : null}
    </div>
  );
}

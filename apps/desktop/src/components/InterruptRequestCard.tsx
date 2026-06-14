import { Check, Loader2, X } from 'lucide-react';
import { useState } from 'react';
import { getIdentity, resolveInterrupt } from '../collab';

type InterruptRequestData = {
  id: string;
  roomId: string;
  sessionId: string;
  requesterId: string;
  requesterNickname: string;
};

type Props = {
  request: InterruptRequestData;
  onResolved: () => void;
};

export function InterruptRequestCard({ request, onResolved }: Props) {
  const [busy, setBusy] = useState(false);

  async function handleResolve(approve: boolean) {
    setBusy(true);
    try {
      const identity = await getIdentity();
      await resolveInterrupt(identity, request.roomId, request.id, approve);
      onResolved();
    } catch {
      setBusy(false);
    }
  }

  return (
    <div className="interrupt-request-card" role="alert">
      <p className="interrupt-request-text">
        <strong>{request.requesterNickname}</strong> wants to interrupt your session
      </p>
      <div className="interrupt-request-actions">
        <button
          type="button"
          className="interrupt-request-approve"
          onClick={() => handleResolve(true)}
          disabled={busy}
        >
          {busy ? <Loader2 size={12} className="spin" /> : <Check size={12} />}
          Allow
        </button>
        <button
          type="button"
          className="interrupt-request-reject"
          onClick={() => handleResolve(false)}
          disabled={busy}
        >
          <X size={12} />
          Deny
        </button>
      </div>
    </div>
  );
}

export type { InterruptRequestData };

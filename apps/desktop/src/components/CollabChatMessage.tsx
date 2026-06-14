import { type ChatMessage } from '../useCollabChat';

type Props = {
  message: ChatMessage;
  isSelf: boolean;
};

export function CollabChatMessage({ message, isSelf }: Props) {
  if (message.fromId === 'system') {
    return <div className="collab-chat-system">{message.text}</div>;
  }

  const roleClass = isSelf ? 'self' : message.role;
  const time = new Date(message.timestamp).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  });

  return (
    <div className={`collab-chat-msg collab-chat-msg-${roleClass}`}>
      <div className="collab-chat-msg-header">
        {!isSelf && (
          <>
            <span className={`collab-chat-badge collab-chat-badge-${message.role}`}>
              {message.role}
            </span>
            <span className={`collab-chat-name collab-chat-name-${message.role}`}>
              {message.nickname}
            </span>
          </>
        )}
        {isSelf && <span className="collab-chat-you">You</span>}
        <span className="collab-chat-time">{time}</span>
      </div>
      <div className={`collab-chat-bubble collab-chat-bubble-${roleClass}`}>
        {message.text}
      </div>
    </div>
  );
}

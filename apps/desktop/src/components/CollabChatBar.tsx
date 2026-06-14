import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { useCollabChat } from '../useCollabChat';
import { CollabChatMessage } from './CollabChatMessage';

type Props = {
  roomId: string | null;
  selfId: string | null;
  isActive: boolean;
  activePromptNickname?: string | null;
  displayName?: string;
};

export function CollabChatBar({ roomId, selfId, isActive, activePromptNickname, displayName }: Props) {
  const { messages, unread, isOpen, openPanel, closePanel, sendMessage } = useCollabChat(
    roomId,
    selfId,
    isActive,
    displayName,
  );
  const [input, setInput] = useState('');
  const bottomRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  useEffect(() => {
    if (isOpen) inputRef.current?.focus();
  }, [isOpen]);

  function handleSend() {
    if (!input.trim()) return;
    sendMessage(input);
    setInput('');
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  if (!isActive) return null;

  return (
    <div className="collab-chat-bar">
      {isOpen && (
        <div className="collab-chat-panel">
          <div className="collab-chat-header">
            <div className="collab-chat-header-left">
              <span className="collab-chat-dot" />
              <span className="collab-chat-title">Session Chat</span>
            </div>
            <button
              onClick={closePanel}
              className="collab-chat-close"
              type="button"
            >
              ×
            </button>
          </div>

          {activePromptNickname && (
            <div className="collab-chat-prompting" role="status">
              <span className="collab-chat-prompting-dot" />
              <span className="collab-chat-prompting-text">
                <strong>{activePromptNickname}</strong> is prompting…
              </span>
            </div>
          )}

          <div className="collab-chat-messages">
            {messages.length === 0 && (
              <div className="collab-chat-empty">
                <span className="collab-chat-empty-icon">💬</span>
                <span className="collab-chat-empty-text">
                  No messages yet.
                  <br />
                  Say hi to your collaborators!
                </span>
              </div>
            )}
            {messages.map((msg) => (
              <CollabChatMessage key={msg.id} message={msg} isSelf={msg.fromId === selfId} />
            ))}
            <div ref={bottomRef} />
          </div>

          <div className="collab-chat-input-bar">
            <input
              ref={inputRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Say something..."
              maxLength={500}
              className="collab-chat-input"
            />
            <button
              onClick={handleSend}
              disabled={!input.trim()}
              className="collab-chat-send"
              type="button"
            >
              ↑
            </button>
          </div>
        </div>
      )}

      <button
        onClick={isOpen ? closePanel : openPanel}
        className={
          activePromptNickname && !isOpen
            ? 'collab-chat-toggle collab-chat-toggle-prompting'
            : 'collab-chat-toggle'
        }
        type="button"
        title={activePromptNickname ? `${activePromptNickname} is prompting…` : undefined}
      >
        <span className="collab-chat-toggle-icon">{isOpen ? '×' : '💬'}</span>
        {!isOpen && unread > 0 && (
          <span className="collab-chat-unread">
            {unread > 9 ? '9+' : unread}
          </span>
        )}
        {!isOpen && activePromptNickname && unread === 0 && (
          <span className="collab-chat-prompting-pulse" aria-hidden="true" />
        )}
      </button>
    </div>
  );
}

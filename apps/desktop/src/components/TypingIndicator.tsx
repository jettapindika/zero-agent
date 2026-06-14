export function TypingIndicator() {
  return (
    <article
      className="message assistant typing"
      role="status"
      aria-label="Zero is thinking"
      aria-live="polite"
    >
      <header className="message-head">
        <p className="role">
          assistant <span className="typing-pill">thinking</span>
        </p>
      </header>
      <div className="message-content">
        <span className="typing-dots" aria-hidden="true">
          <span className="typing-dot" />
          <span className="typing-dot" />
          <span className="typing-dot" />
        </span>
      </div>
    </article>
  );
}

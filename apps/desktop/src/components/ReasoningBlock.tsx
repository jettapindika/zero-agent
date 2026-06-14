type Props = {
  lines: string[];
};

// A single reasoning quote-block. Multiple consecutive "Reasoning:" lines fold
// into one block so multi-sentence reasoning reads as one paragraph.
export function ReasoningBlock({ lines }: Props) {
  return (
    <blockquote className="reasoning-block">
      <span className="reasoning-label">Reasoning</span>
      <span className="reasoning-body">
        {lines.map((line, idx) => (
          <span key={idx}>{line}{idx < lines.length - 1 ? ' ' : ''}</span>
        ))}
      </span>
    </blockquote>
  );
}

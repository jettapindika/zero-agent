type Props = {
  prefix: string;
  title: string;
};

// Phase header — "Phase 1.2:" portion neutral, title pink-ish so phase
// boundaries are easy to scan.
export function PhaseHeader({ prefix, title }: Props) {
  return (
    <h3 className="phase-header">
      <span className="phase-prefix">{prefix}</span>
      <span className="phase-title">{title}</span>
    </h3>
  );
}

import type { ActionLine as ActionLineData } from '../chat/parse';

type Props = {
  lines: ActionLineData[];
};

// Renders a tight stack of "→ Verb path [meta]" rows. Consecutive rows share
// the same accent border so the eye reads them as one ledger.
export function ActionLineGroup({ lines }: Props) {
  return (
    <div className="action-group">
      {lines.map((line, idx) => (
        <div className="action-row" key={idx}>
          <span className="action-arrow">{line.arrow}</span>
          <span className="action-verb">{line.verb}</span>
          <span className="action-body">{line.body}</span>
          {line.meta ? <span className="action-meta">{line.meta}</span> : null}
        </div>
      ))}
    </div>
  );
}

import type { ReactNode } from 'react';
import type { BulletLine } from '../chat/parse';

type Props = {
  lines: BulletLine[];
  renderInline: (text: string) => ReactNode;
};

// Flat bullet list. Each line is "- Label: body". Label is bold-ish, body is
// muted, inline backtick code is colored by the existing renderInlineRichText
// passed in from the parent so styling stays consistent across all blocks.
export function BulletItemList({ lines, renderInline }: Props) {
  return (
    <ul className="bullet-list">
      {lines.map((line, idx) => (
        <li className="bullet-row" key={idx}>
          <span className="bullet-dash">-</span>
          <span className="bullet-label">{line.label}:</span>
          <span className="bullet-body">{renderInline(line.body)}</span>
        </li>
      ))}
    </ul>
  );
}

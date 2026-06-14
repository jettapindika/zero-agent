import type { StepLine, StepStatus } from '../chat/parse';

type Props = {
  lines: StepLine[];
};

const STATUS_GLYPH: Record<StepStatus, string> = {
  done: '✓',
  in_progress: '●',
  pending: '○',
  failed: '✗',
  warn: '⚠',
  unknown: '·',
};

// Splits "label — tools: `walk` — risk: low, needs permission" into a
// structured row so the renderer can color tool names, risk tier, and the
// permission badge independently.
function decomposeStep(body: string): {
  label: string;
  tools?: string[];
  risk?: 'low' | 'med' | 'high';
  needsPermission: boolean;
} {
  const segments = body.split(/\s+—\s+/);
  const label = segments[0] ?? '';
  let tools: string[] | undefined;
  let risk: 'low' | 'med' | 'high' | undefined;
  let needsPermission = false;

  for (const segRaw of segments.slice(1)) {
    const seg = segRaw.trim();
    if (/^tools:\s*/i.test(seg)) {
      const inner = seg.replace(/^tools:\s*/i, '');
      if (/^none\b/i.test(inner)) {
        tools = [];
      } else {
        tools = [...inner.matchAll(/`([^`]+)`/g)].map((m) => m[1]);
        if (tools.length === 0 && inner.trim() !== '') tools = [inner.trim()];
      }
    } else if (/^risk:\s*/i.test(seg)) {
      const inner = seg.replace(/^risk:\s*/i, '').toLowerCase();
      if (/needs permission/.test(inner)) needsPermission = true;
      const head = inner.split(',')[0]?.trim();
      if (head === 'low' || head === 'med' || head === 'high') risk = head;
    } else if (/needs permission/i.test(seg)) {
      needsPermission = true;
    }
  }

  return { label, tools, risk, needsPermission };
}

export function StepList({ lines }: Props) {
  return (
    <ol className="step-list">
      {lines.map((step, idx) => {
        const meta = decomposeStep(step.body);
        return (
          <li className={`step-row status-${step.status}`} key={idx}>
            <span className="step-index">[{step.index}]</span>
            <span className={`step-glyph status-${step.status}`} aria-hidden="true">
              {STATUS_GLYPH[step.status]}
            </span>
            <span className="step-label">{meta.label}</span>
            {meta.tools !== undefined ? (
              <span className="step-meta">
                <span className="step-meta-label">— tools:</span>
                {meta.tools.length === 0 ? (
                  <span className="step-tool-none">none</span>
                ) : (
                  meta.tools.map((t, i) => (
                    <code className="step-tool" key={i}>{t}</code>
                  ))
                )}
              </span>
            ) : null}
            {meta.risk ? (
              <span className="step-meta">
                <span className="step-meta-label">— risk:</span>
                <span className={`step-risk risk-${meta.risk}`}>{meta.risk}</span>
              </span>
            ) : null}
            {meta.needsPermission ? (
              <span className="step-badge">needs permission</span>
            ) : null}
          </li>
        );
      })}
    </ol>
  );
}

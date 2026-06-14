import { useState, type ReactNode } from 'react';
import { AlertCircle, ChevronDown, ChevronRight } from 'lucide-react';
import type { Part } from '../zero-api';
import { buildActionLine, classifyToolResult } from '../chat/tool-render';

const TERMINAL_COLLAPSE_LINES = 8;
const TERMINAL_TOTAL_THRESHOLD = 20;

type RenderCode = (language: string, lines: string[]) => ReactNode;

type Props = {
  part: Part;
  originCall?: Part;
  renderCode: RenderCode;
};

export function ToolPart({ part, originCall, renderCode }: Props) {
  if (part.type === 'tool_call') {
    return <ToolCallRow part={part} />;
  }
  if (part.type === 'tool_result') {
    return <ToolResultBlock part={part} originCall={originCall} renderCode={renderCode} />;
  }
  return null;
}

function ToolCallRow({ part }: { part: Part }) {
  const line = buildActionLine(part);
  return (
    <div className="action-group tool-call">
      <div className="action-row">
        <span className="action-arrow">→</span>
        <span className="action-verb">{line.verb}</span>
        {line.path ? <span className="action-body">{line.path}</span> : null}
        {line.command ? (
          <code className="action-body tool-command">{line.command}</code>
        ) : null}
        {line.meta ? <span className="action-meta">{line.meta}</span> : null}
      </div>
    </div>
  );
}

function ToolResultBlock({
  part,
  originCall,
  renderCode,
}: {
  part: Part;
  originCall?: Part;
  renderCode: RenderCode;
}) {
  const block = classifyToolResult(part, originCall);

  if (block.kind === 'silent') return null;

  if (block.kind === 'error') {
    return (
      <div className="tool-error" role="status">
        <AlertCircle size={14} className="tool-error-icon" />
        <pre className="tool-error-message">{block.content}</pre>
      </div>
    );
  }

  if (block.kind === 'code') {
    return (
      <div className="tool-output">
        {renderCode(block.language, block.content.split('\n'))}
      </div>
    );
  }

  if (block.kind === 'json') {
    return (
      <div className="tool-output">
        {renderCode('json', block.content.split('\n'))}
      </div>
    );
  }

  return <TerminalBlock content={block.content} />;
}

function TerminalBlock({ content }: { content: string }) {
  const lines = content.replace(/\s+$/, '').split('\n');
  const overflow = lines.length > TERMINAL_TOTAL_THRESHOLD;
  const [expanded, setExpanded] = useState(!overflow);

  const visibleLines = expanded ? lines : lines.slice(0, TERMINAL_COLLAPSE_LINES);
  const hiddenCount = lines.length - visibleLines.length;

  return (
    <div className="terminal-block">
      <pre className="terminal-pre">
        {visibleLines.map((line, i) => (
          <code key={i} className="terminal-line">
            {line || ' '}
          </code>
        ))}
      </pre>
      {overflow ? (
        <button
          type="button"
          className="terminal-toggle"
          onClick={() => setExpanded((x) => !x)}
        >
          {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
          {expanded ? 'Collapse' : `Show ${hiddenCount} more line${hiddenCount === 1 ? '' : 's'}`}
        </button>
      ) : null}
    </div>
  );
}

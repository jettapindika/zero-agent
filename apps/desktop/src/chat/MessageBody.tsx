import type { ReactNode } from 'react';
import { ActionLineGroup } from '../components/ActionLine';
import { BulletItemList } from '../components/BulletItem';
import { PhaseHeader } from '../components/PhaseHeader';
import { ReasoningBlock } from '../components/ReasoningBlock';
import { StepList } from '../components/StepList';
import { TaskLog } from '../components/TaskLog';
import { parseAssistantText, type Block } from './parse';

// Detect whether the existing simple text/code parser is good enough for this
// message. If none of the new markers are present we keep the legacy path so
// historical chats render exactly the same.
const NEW_MARKERS = /(^|\n)\s*(→ |\[task\]|\[\d+\]|Phase\s+[\d.]+:|Reasoning:|-\s+[A-Za-z][A-Za-z0-9 _-]{0,40}:)/i;

export function shouldUseStructuredRenderer(text: string): boolean {
  return NEW_MARKERS.test(text);
}

type Props = {
  text: string;
  // Reuse the existing app-level renderers so coloring stays consistent.
  renderInline: (text: string) => ReactNode;
  renderCode: (language: string, lines: string[]) => ReactNode;
  renderProseLine: (line: string) => ReactNode;
};

export function MessageBody({ text, renderInline, renderCode, renderProseLine }: Props) {
  const blocks = parseAssistantText(text);

  // [task] blocks always render last so they never compete with main content.
  const main = blocks.filter((b) => b.type !== 'task');
  const taskBlocks = blocks.filter((b): b is Extract<Block, { type: 'task' }> => b.type === 'task');
  const allTasks = taskBlocks.flatMap((b) => b.lines);

  return (
    <div className="message-body">
      {main.map((block, idx) => renderBlock(block, idx, renderInline, renderCode, renderProseLine))}
      {allTasks.length > 0 ? <TaskLog lines={allTasks} /> : null}
    </div>
  );
}

function renderBlock(
  block: Block,
  idx: number,
  renderInline: (text: string) => ReactNode,
  renderCode: (language: string, lines: string[]) => ReactNode,
  renderProseLine: (line: string) => ReactNode,
): ReactNode {
  switch (block.type) {
    case 'code':
      return <div key={idx}>{renderCode(block.language, block.lines)}</div>;
    case 'action':
      return <ActionLineGroup key={idx} lines={block.lines} />;
    case 'step':
      return <StepList key={idx} lines={block.lines} />;
    case 'phase':
      return <PhaseHeader key={idx} prefix={block.prefix} title={block.title} />;
    case 'reasoning':
      return <ReasoningBlock key={idx} lines={block.lines} />;
    case 'bullet':
      return <BulletItemList key={idx} lines={block.lines} renderInline={renderInline} />;
    case 'prose':
      return (
        <div className="prose-block" key={idx}>
          {block.lines.map((line, i) => (
            <span key={i} className="prose-line">{renderProseLine(line)}</span>
          ))}
        </div>
      );
    case 'task':
      return null; // handled separately so [task] always sinks to the bottom
  }
}

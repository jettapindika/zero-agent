// Line-based parser for assistant messages. Walks the answer top-to-bottom,
// classifies each line, and groups consecutive same-type lines into one
// Block. The renderer in MessageBody.tsx maps each Block kind to a component.
//
// Recognized line shapes:
//   ```lang ... ```   -> code
//   → ...             -> action
//   [task] ...        -> task log
//   [N] ...           -> numbered step
//   Phase N.N: ...    -> phase header
//   Reasoning: ...    -> reasoning prose
//   - Label: ...      -> bullet item (single dash + label-colon prefix)
//   anything else     -> prose

export type StepStatus = 'done' | 'in_progress' | 'pending' | 'failed' | 'warn' | 'unknown';

export type ActionLine = {
  arrow: string;
  verb: string;
  body: string;
  meta?: string;
};

export type StepLine = {
  index: string;
  status: StepStatus;
  body: string;
};

export type BulletLine = {
  label: string;
  body: string;
};

export type Block =
  | { type: 'code'; language: string; lines: string[] }
  | { type: 'action'; lines: ActionLine[] }
  | { type: 'task'; lines: string[] }
  | { type: 'step'; lines: StepLine[] }
  | { type: 'phase'; prefix: string; title: string }
  | { type: 'reasoning'; lines: string[] }
  | { type: 'bullet'; lines: BulletLine[] }
  | { type: 'prose'; lines: string[] };

const FENCE_RE = /^```\s*([\w.+-]*)\s*$/;
const ACTION_RE = /^→\s+(\S+)\s+(.*?)(?:\s+(\[[^\]]+\]))?\s*$/;
const TASK_RE = /^\[task\]\s+(.+)$/i;
const STEP_RE = /^\[(\d+)\]\s+(.*)$/;
const PHASE_RE = /^(Phase\s+[\d.]+):\s*(.*)$/i;
const REASONING_RE = /^Reasoning:\s*(.*)$/i;
const BULLET_RE = /^-\s+([A-Za-z][A-Za-z0-9 _-]{0,40}):\s*(.*)$/;

const STATUS_MAP: Record<string, StepStatus> = {
  '✓': 'done',
  '●': 'in_progress',
  '○': 'pending',
  '✗': 'failed',
  '⚠': 'warn',
};

function classifyStepBody(body: string): { status: StepStatus; rest: string } {
  const trimmed = body.trimStart();
  const first = Array.from(trimmed)[0] ?? '';
  const status = STATUS_MAP[first];
  if (status) {
    // Drop the symbol + a single following space.
    return { status, rest: trimmed.slice(first.length).trimStart() };
  }
  return { status: 'unknown', rest: trimmed };
}

export function parseAssistantText(input: string): Block[] {
  const blocks: Block[] = [];
  const lines = input.split(/\r?\n/);

  let inCode = false;
  let codeLang = '';
  let codeBuf: string[] = [];

  let pendingTaskBuf: string[] = [];
  let pendingProseBuf: string[] = [];
  let pendingActionBuf: ActionLine[] = [];
  let pendingStepBuf: StepLine[] = [];
  let pendingReasoningBuf: string[] = [];
  let pendingBulletBuf: BulletLine[] = [];

  function flushAll() {
    if (pendingActionBuf.length) {
      blocks.push({ type: 'action', lines: pendingActionBuf });
      pendingActionBuf = [];
    }
    if (pendingStepBuf.length) {
      blocks.push({ type: 'step', lines: pendingStepBuf });
      pendingStepBuf = [];
    }
    if (pendingBulletBuf.length) {
      blocks.push({ type: 'bullet', lines: pendingBulletBuf });
      pendingBulletBuf = [];
    }
    if (pendingReasoningBuf.length) {
      blocks.push({ type: 'reasoning', lines: pendingReasoningBuf });
      pendingReasoningBuf = [];
    }
    if (pendingProseBuf.length) {
      blocks.push({ type: 'prose', lines: pendingProseBuf });
      pendingProseBuf = [];
    }
    if (pendingTaskBuf.length) {
      blocks.push({ type: 'task', lines: pendingTaskBuf });
      pendingTaskBuf = [];
    }
  }

  for (const raw of lines) {
    const line = raw;

    // Fenced code: open / close
    const fence = line.match(FENCE_RE);
    if (fence) {
      flushAll();
      if (inCode) {
        blocks.push({ type: 'code', language: codeLang || 'text', lines: codeBuf });
        inCode = false;
        codeLang = '';
        codeBuf = [];
      } else {
        inCode = true;
        codeLang = fence[1] || 'text';
        codeBuf = [];
      }
      continue;
    }
    if (inCode) {
      codeBuf.push(line);
      continue;
    }

    // Action lines: "→ Verb path/to/file [offset=N, limit=N]"
    const action = line.match(ACTION_RE);
    if (action) {
      // Flush any non-action buffers; allow consecutive action lines to group.
      if (pendingStepBuf.length || pendingBulletBuf.length || pendingReasoningBuf.length || pendingProseBuf.length || pendingTaskBuf.length) {
        flushAll();
      }
      pendingActionBuf.push({
        arrow: '→',
        verb: action[1],
        body: action[2],
        meta: action[3],
      });
      continue;
    }

    // [task] lines: collect at end of a section, separately from main flow.
    const task = line.match(TASK_RE);
    if (task) {
      // Flush non-task content but keep accumulating tasks.
      if (pendingActionBuf.length || pendingStepBuf.length || pendingBulletBuf.length || pendingReasoningBuf.length || pendingProseBuf.length) {
        flushAll();
      }
      pendingTaskBuf.push(task[1].trim());
      continue;
    }

    // [N] numbered steps
    const step = line.match(STEP_RE);
    if (step) {
      if (pendingActionBuf.length || pendingBulletBuf.length || pendingReasoningBuf.length || pendingProseBuf.length || pendingTaskBuf.length) {
        flushAll();
      }
      const { status, rest } = classifyStepBody(step[2]);
      pendingStepBuf.push({ index: step[1], status, body: rest });
      continue;
    }

    // Phase N.N: title
    const phase = line.match(PHASE_RE);
    if (phase) {
      flushAll();
      blocks.push({ type: 'phase', prefix: `${phase[1]}:`, title: phase[2].trim() });
      continue;
    }

    // Reasoning: prose
    const reasoning = line.match(REASONING_RE);
    if (reasoning) {
      if (pendingActionBuf.length || pendingStepBuf.length || pendingBulletBuf.length || pendingProseBuf.length || pendingTaskBuf.length) {
        flushAll();
      }
      pendingReasoningBuf.push(reasoning[1]);
      continue;
    }

    // Bullet items: "- Label: body"
    const bullet = line.match(BULLET_RE);
    if (bullet) {
      if (pendingActionBuf.length || pendingStepBuf.length || pendingReasoningBuf.length || pendingProseBuf.length || pendingTaskBuf.length) {
        flushAll();
      }
      pendingBulletBuf.push({ label: bullet[1], body: bullet[2] });
      continue;
    }

    // Default: prose. Empty lines flush an in-progress prose group so paragraph
    // breaks remain as separate prose blocks.
    if (line.trim() === '') {
      if (pendingProseBuf.length) {
        blocks.push({ type: 'prose', lines: pendingProseBuf });
        pendingProseBuf = [];
      } else if (pendingActionBuf.length || pendingStepBuf.length || pendingReasoningBuf.length || pendingBulletBuf.length || pendingTaskBuf.length) {
        flushAll();
      }
      continue;
    }
    if (pendingActionBuf.length || pendingStepBuf.length || pendingReasoningBuf.length || pendingBulletBuf.length || pendingTaskBuf.length) {
      flushAll();
    }
    pendingProseBuf.push(line);
  }

  // EOF: flush whatever is buffered. Unclosed code block becomes a code block
  // anyway so partial streaming output still renders.
  if (inCode && codeBuf.length > 0) {
    blocks.push({ type: 'code', language: codeLang || 'text', lines: codeBuf });
  }
  flushAll();

  return blocks;
}

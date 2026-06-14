import type { Part } from '../zero-api';

export type ActionLineModel = {
  verb: string;
  path?: string;
  command?: string;
  meta?: string;
};

export type OutputBlock =
  | { kind: 'silent' }
  | { kind: 'error'; content: string }
  | { kind: 'json'; content: string }
  | { kind: 'code'; content: string; language: string; filename?: string }
  | { kind: 'terminal'; content: string };

const VERB_BY_TOOL: Record<string, string> = {
  read: 'Read',
  read_file: 'Read',
  write: 'Write',
  write_file: 'Write',
  edit: 'Edit',
  edit_file: 'Edit',
  delete: 'Delete',
  delete_file: 'Delete',
  bash: 'Run',
  shell: 'Run',
  run: 'Run',
  ls: 'Inspect',
  walk: 'Inspect',
  list: 'Inspect',
  glob: 'Search',
  grep: 'Search',
  search: 'Search',
  fetch: 'Fetch',
  http: 'Fetch',
  attach_read: 'Read',
};

const LANG_BY_EXT: Record<string, string> = {
  go: 'go',
  ts: 'typescript',
  tsx: 'typescript',
  js: 'javascript',
  jsx: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  py: 'python',
  rs: 'rust',
  java: 'java',
  kt: 'kotlin',
  swift: 'swift',
  rb: 'ruby',
  php: 'php',
  c: 'c',
  h: 'c',
  cpp: 'cpp',
  hpp: 'cpp',
  cs: 'csharp',
  json: 'json',
  yaml: 'yaml',
  yml: 'yaml',
  toml: 'toml',
  md: 'markdown',
  sh: 'bash',
  bash: 'bash',
  zsh: 'bash',
  sql: 'sql',
  html: 'html',
  css: 'css',
  scss: 'scss',
  xml: 'xml',
};

export function detectLang(pathOrEmpty: string): string | undefined {
  if (!pathOrEmpty) return undefined;
  const m = pathOrEmpty.match(/\.([A-Za-z0-9]+)(?:\?|#|$)/);
  if (!m) return undefined;
  return LANG_BY_EXT[m[1].toLowerCase()];
}

const SILENT_RESULTS = new Set(['ok', 'done', 'success', 'null', '{}', '[]', '', 'undefined']);

export function isSilent(content: string): boolean {
  return SILENT_RESULTS.has(content.toLowerCase().trim());
}

export function parseJsonSafe(raw: string | null | undefined): unknown {
  if (!raw) return undefined;
  try {
    return JSON.parse(raw);
  } catch {
    return undefined;
  }
}

// Backend wraps tool output in {title, output, isError}. We unwrap so the UI
// shows just the underlying content; if the envelope shape doesn't match (e.g.
// marshal-failed fallback), we treat the whole string as the output verbatim.
export function unwrapToolResult(toolResultJson: string | null | undefined): {
  output: string;
  title?: string;
  isError: boolean;
} {
  if (!toolResultJson) return { output: '', isError: false };
  const parsed = parseJsonSafe(toolResultJson);
  if (
    parsed &&
    typeof parsed === 'object' &&
    'output' in parsed &&
    typeof (parsed as { output: unknown }).output === 'string'
  ) {
    const obj = parsed as { output: string; title?: string; isError?: boolean };
    return {
      output: obj.output,
      title: typeof obj.title === 'string' ? obj.title : undefined,
      isError: Boolean(obj.isError),
    };
  }
  return { output: toolResultJson, isError: false };
}

export function buildActionLine(part: Part): ActionLineModel {
  const tool = part.toolName ?? '';
  const verb = VERB_BY_TOOL[tool] ?? capitalize(tool || 'Tool');
  const args = (parseJsonSafe(part.toolArgsJson) as Record<string, unknown>) ?? {};

  const path = pickString(args, ['path', 'file', 'filePath']);
  const command = pickString(args, ['command', 'cmd']);
  const url = pickString(args, ['url']);
  const pattern = pickString(args, ['pattern']);
  const id = pickString(args, ['id']);

  const meta = buildMeta(args, tool);

  return {
    verb,
    path: path ?? url ?? pattern ?? id,
    command,
    meta,
  };
}

function buildMeta(args: Record<string, unknown>, tool: string): string | undefined {
  const parts: string[] = [];
  pushIfPresent(parts, args, 'startLine');
  pushIfPresent(parts, args, 'endLine');
  pushIfPresent(parts, args, 'offset');
  pushIfPresent(parts, args, 'limit');
  pushIfPresent(parts, args, 'line');
  pushIfPresent(parts, args, 'maxDepth');
  pushIfPresent(parts, args, 'maxFiles');
  pushIfPresent(parts, args, 'maxResults');
  if (tool === 'fetch') {
    const method = pickString(args, ['method']);
    if (method && method.toUpperCase() !== 'GET') parts.push(`method=${method.toUpperCase()}`);
  }
  if (tool === 'edit' && args.global === true) parts.push('global');
  if (tool === 'ls' && args.recursive === true) parts.push('recursive');
  if (tool === 'attach_read' && args.list === true) parts.push('list');
  if (tool === 'attach_read') {
    const chunk = args.chunk;
    if (typeof chunk === 'number' && chunk > 0) parts.push(`chunk=${chunk}`);
  }
  return parts.length > 0 ? `[${parts.join(', ')}]` : undefined;
}

function pushIfPresent(out: string[], args: Record<string, unknown>, key: string) {
  const v = args[key];
  if (typeof v === 'number' && Number.isFinite(v)) out.push(`${key}=${v}`);
}

function pickString(args: Record<string, unknown>, keys: string[]): string | undefined {
  for (const k of keys) {
    const v = args[k];
    if (typeof v === 'string' && v.length > 0) return v;
  }
  return undefined;
}

function capitalize(s: string): string {
  return s.length === 0 ? s : s[0].toUpperCase() + s.slice(1);
}

export function classifyToolResult(part: Part, originCall?: Part): OutputBlock {
  const { output, isError } = unwrapToolResult(part.toolResultJson);

  if (part.isError || isError) {
    return { kind: 'error', content: output || '(no error message)' };
  }
  if (isSilent(output)) {
    return { kind: 'silent' };
  }

  const trimmed = output.trim();
  if (looksLikeJson(trimmed)) {
    try {
      const parsed = JSON.parse(trimmed);
      return { kind: 'json', content: JSON.stringify(parsed, null, 2) };
    } catch {}
  }

  const callArgs =
    (parseJsonSafe(originCall?.toolArgsJson) as Record<string, unknown>) ?? {};
  const filename = pickString(callArgs, ['path', 'file', 'filePath']);
  const lang = detectLang(filename ?? '');

  const isReadLike =
    part.toolName === 'read' ||
    part.toolName === 'read_file' ||
    part.toolName === 'attach_read';

  if ((isReadLike && lang) || looksLikeCode(trimmed)) {
    return {
      kind: 'code',
      content: output,
      language: lang ?? 'plaintext',
      filename,
    };
  }

  return { kind: 'terminal', content: output };
}

function looksLikeJson(s: string): boolean {
  if (s.length < 2) return false;
  const first = s[0];
  const last = s[s.length - 1];
  return (first === '{' && last === '}') || (first === '[' && last === ']');
}

function looksLikeCode(s: string): boolean {
  return /^(package |import |func |const |type |class |def |fn |pub |use |from )/m.test(s);
}

export type DroppedKind =
  | 'IMG'
  | 'VID'
  | 'AUD'
  | 'PDF'
  | 'DOC'
  | 'XLS'
  | 'PPT'
  | 'TXT'
  | 'FILE';

export type DroppedFile = {
  path: string;
  name: string;
  kind: DroppedKind;
};

const EXT_TO_KIND: Record<string, DroppedKind> = {
  png: 'IMG', jpg: 'IMG', jpeg: 'IMG', webp: 'IMG', gif: 'IMG',
  bmp: 'IMG', svg: 'IMG', heic: 'IMG', tiff: 'IMG',

  mp4: 'VID', mov: 'VID', webm: 'VID', mkv: 'VID', avi: 'VID',
  m4v: 'VID', flv: 'VID', wmv: 'VID',

  mp3: 'AUD', wav: 'AUD', ogg: 'AUD', m4a: 'AUD', flac: 'AUD',
  aac: 'AUD', opus: 'AUD',

  pdf: 'PDF',
  doc: 'DOC', docx: 'DOC', odt: 'DOC', rtf: 'DOC',
  xls: 'XLS', xlsx: 'XLS', csv: 'XLS', tsv: 'XLS', ods: 'XLS',
  ppt: 'PPT', pptx: 'PPT', odp: 'PPT', key: 'PPT',

  txt: 'TXT', md: 'TXT', mdx: 'TXT', log: 'TXT',
  json: 'TXT', yaml: 'TXT', yml: 'TXT', toml: 'TXT', xml: 'TXT',
  html: 'TXT', htm: 'TXT', css: 'TXT', scss: 'TXT', less: 'TXT',
  js: 'TXT', jsx: 'TXT', ts: 'TXT', tsx: 'TXT', mjs: 'TXT', cjs: 'TXT',
  go: 'TXT', rs: 'TXT', py: 'TXT', rb: 'TXT', php: 'TXT', java: 'TXT',
  kt: 'TXT', swift: 'TXT', c: 'TXT', h: 'TXT', cpp: 'TXT', hpp: 'TXT',
  cs: 'TXT', sh: 'TXT', bash: 'TXT', zsh: 'TXT', fish: 'TXT',
  sql: 'TXT', graphql: 'TXT', proto: 'TXT', env: 'TXT', conf: 'TXT',
  ini: 'TXT', dockerfile: 'TXT', makefile: 'TXT',
};

export function detectKind(path: string): DroppedKind {
  const slash = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
  const base = slash === -1 ? path : path.slice(slash + 1);
  const lower = base.toLowerCase();

  if (lower === 'dockerfile' || lower === 'makefile') return 'TXT';

  const dot = lower.lastIndexOf('.');
  if (dot === -1) return 'FILE';
  const ext = lower.slice(dot + 1);
  return EXT_TO_KIND[ext] ?? 'FILE';
}

export function basename(path: string): string {
  const slash = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
  return slash === -1 ? path : path.slice(slash + 1);
}

export function toDroppedFile(path: string): DroppedFile {
  return { path, name: basename(path), kind: detectKind(path) };
}

export function formatTagged(file: DroppedFile): string {
  return `[${file.kind}] ${file.path}`;
}

export function dedupePaths(existing: DroppedFile[], next: DroppedFile[]): DroppedFile[] {
  const seen = new Set(existing.map((f) => f.path));
  const out = [...existing];
  for (const f of next) {
    if (!seen.has(f.path)) {
      seen.add(f.path);
      out.push(f);
    }
  }
  return out;
}

// Auto-suggest popover that floats above the composer when the user starts a
// prompt with "/". Filters by partial command name.

const COMMANDS: { name: string; usage: string; description: string }[] = [
  { name: '/model', usage: '/model [provider/name]', description: 'Switch the active model. No args opens a picker.' },
  { name: '/agent', usage: '/agent <build|plan|explore>', description: 'Switch the active agent mode.' },
  { name: '/clear', usage: '/clear', description: 'Clear the prompt queue (does not delete history).' },
  { name: '/help', usage: '/help', description: 'Show all slash commands.' },
];

export type SlashPreviewProps = {
  input: string;
  onPick: (command: string) => void;
};

export function SlashPreview({ input, onPick }: SlashPreviewProps) {
  const trimmed = input.trim();
  if (!trimmed.startsWith('/')) return null;

  const head = trimmed.split(/\s+/)[0]?.toLowerCase() ?? '';
  // Once the user has typed args (a space + something), hide the preview.
  if (trimmed.includes(' ')) return null;

  const matches = COMMANDS.filter((cmd) => cmd.name.startsWith(head));
  if (matches.length === 0) return null;

  return (
    <div className="slash-preview" role="listbox" aria-label="Slash commands">
      <p className="slash-preview-title">Slash commands</p>
      {matches.map((cmd) => (
        <button
          className="slash-preview-row"
          key={cmd.name}
          onClick={() => onPick(cmd.name)}
          type="button"
        >
          <div>
            <p className="slash-preview-label">{cmd.usage}</p>
            <p className="slash-preview-detail">{cmd.description}</p>
          </div>
        </button>
      ))}
      <p className="slash-preview-footer">Tip: press Tab to complete · Enter to send · Esc to dismiss.</p>
    </div>
  );
}

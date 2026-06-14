// Slash-command parser for the desktop composer. Returns null when the input
// is a normal prompt and should go to the LLM.

export type SlashCommand = {
  name: 'model' | 'agent' | 'help' | 'clear';
  args: string[];
  raw: string;
};

const SUPPORTED = new Set<SlashCommand['name']>(['model', 'agent', 'help', 'clear']);

export function parseSlashCommand(input: string): SlashCommand | null {
  const trimmed = input.trim();
  if (!trimmed.startsWith('/')) return null;

  const tokens = trimmed.slice(1).split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return null;

  const name = tokens[0].toLowerCase() as SlashCommand['name'];
  if (!SUPPORTED.has(name)) return null;

  return { name, args: tokens.slice(1), raw: trimmed };
}

export type SlashError = { kind: 'error'; message: string };
export type SlashResult =
  | { kind: 'message'; level: 'info' | 'error'; text: string }
  | { kind: 'noop' };

// validateModelId enforces the `provider/name` shape requested in plan question
// 6 (full provider-prefixed IDs only).
export function validateModelId(model: string): SlashError | null {
  if (!model || !model.includes('/')) {
    return {
      kind: 'error',
      message: `Model "${model || '(empty)'}" must be in provider/name form, e.g. openai/gpt-4o-mini.`,
    };
  }
  return null;
}

export const SLASH_HELP_TEXT = [
  'Available slash commands:',
  '  /model <provider/name>   Switch the active model (next turn).',
  '  /agent <build|plan|explore>   Switch the active agent mode.',
  '  /help                    Show this help.',
  '  /clear                   Clear the prompt queue (does not delete history).',
].join('\n');

import { useEffect, useMemo, useRef, useState } from 'react';
import { X } from 'lucide-react';
import { listModels } from '../zero-api';

// Curated fallback list shown when the live `/providers/models` proxy is
// unreachable (provider down, no API key set, network blocked). The real
// model list comes from whatever endpoint the user configured via
// ZERO_ROUTER_BASE_URL — these are just well-known ids users are likely
// to recognize so picking one isn't a dead end.
const FALLBACK_MODELS: ModelEntry[] = [
  { id: 'gpt-4o-mini', provider: 'OpenAI', label: 'GPT-4o mini', description: 'Small, fast, cheap. Good default.' },
  { id: 'gpt-4o', provider: 'OpenAI', label: 'GPT-4o' },
  { id: 'gpt-4.1', provider: 'OpenAI', label: 'GPT-4.1' },
  { id: 'claude-3-5-sonnet-latest', provider: 'Anthropic', label: 'Claude 3.5 Sonnet', description: 'Use via OpenRouter / LiteLLM proxy.' },
  { id: 'llama3.1', provider: 'Ollama', label: 'Llama 3.1 (local)', description: 'Set ZERO_ROUTER_BASE_URL to your Ollama /v1 endpoint.' },
];

type ModelEntry = {
  id: string;
  provider: string;
  label: string;
  description?: string;
};

function inferProvider(modelId: string): string {
  const slash = modelId.indexOf('/');
  if (slash <= 0) {
    if (modelId.startsWith('gpt-') || modelId.startsWith('o1-') || modelId.startsWith('o3-')) return 'OpenAI';
    if (modelId.startsWith('claude-')) return 'Anthropic';
    if (modelId.startsWith('llama') || modelId.startsWith('mistral') || modelId.startsWith('qwen')) return 'Local';
    return 'provider';
  }
  const prefix = modelId.slice(0, slash).toLowerCase();
  switch (prefix) {
    case 'openai':
      return 'OpenAI';
    case 'anthropic':
      return 'Anthropic';
    case 'ollama':
      return 'Ollama';
    case 'openrouter':
      return 'OpenRouter';
    case 'litellm':
      return 'LiteLLM';
    default:
      return prefix;
  }
}

function inferLabel(modelId: string): string {
  const slash = modelId.indexOf('/');
  if (slash <= 0) return modelId;
  return modelId.slice(slash + 1);
}

export type ModelPickerModalProps = {
  open: boolean;
  currentModel: string;
  onClose: () => void;
  onSelect: (modelId: string) => void;
};

export function ModelPickerModal({ open, currentModel, onClose, onSelect }: ModelPickerModalProps) {
  const [query, setQuery] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const [models, setModels] = useState<ModelEntry[]>(FALLBACK_MODELS);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!open) return;
    setQuery('');
    setActiveIndex(0);
    setLoadError(null);
    // Defer focus so transition doesn't steal it.
    const focusId = window.setTimeout(() => inputRef.current?.focus(), 30);

    let cancelled = false;
    setLoading(true);
    listModels()
      .then((live) => {
        if (cancelled) return;
        if (!live || live.length === 0) {
          setModels(FALLBACK_MODELS);
          setLoadError('Provider returned no models; showing curated fallback.');
          return;
        }
        const mapped: ModelEntry[] = live.map((m) => ({
          id: m.id,
          provider: inferProvider(m.id),
          label: m.name || inferLabel(m.id),
        }));
        setModels(mapped);
      })
      .catch((err: Error) => {
        if (cancelled) return;
        setModels(FALLBACK_MODELS);
        setLoadError(`Live model list unreachable: ${err.message}. Showing curated fallback.`);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
      window.clearTimeout(focusId);
    };
  }, [open]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return models;
    return models.filter(
      (m) => m.id.toLowerCase().includes(q) || m.label.toLowerCase().includes(q) || m.provider.toLowerCase().includes(q),
    );
  }, [query, models]);

  useEffect(() => {
    if (activeIndex >= filtered.length) setActiveIndex(0);
  }, [filtered.length, activeIndex]);

  if (!open) return null;

  function commit(modelId: string) {
    onSelect(modelId);
    onClose();
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Escape') {
      onClose();
      return;
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setActiveIndex((i) => Math.min(filtered.length - 1, i + 1));
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActiveIndex((i) => Math.max(0, i - 1));
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      const target = filtered[activeIndex];
      if (target) commit(target.id);
    }
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <header className="modal-header">
          <div>
            <p className="eyebrow">Switch model</p>
            <h2>{loading ? 'Loading models…' : `Select an AI model (${models.length})`}</h2>
          </div>
          <button aria-label="Close" className="modal-close" onClick={onClose} type="button">
            <X size={16} />
          </button>
        </header>
        <input
          className="modal-search"
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={loading ? 'Loading…' : 'Filter by name, id, or provider…'}
          ref={inputRef}
          value={query}
        />
        {loadError ? <p className="modal-warn">{loadError}</p> : null}
        <ul className="modal-list">
          {filtered.length === 0 ? <li className="modal-empty">No models match &quot;{query}&quot;.</li> : null}
          {filtered.map((model, index) => {
            const isCurrent = model.id === currentModel;
            const isActive = index === activeIndex;
            return (
              <li className={`modal-row ${isActive ? 'active' : ''} ${isCurrent ? 'current' : ''}`} key={model.id}>
                <button onClick={() => commit(model.id)} type="button">
                  <div>
                    <p className="modal-row-label">{model.label}</p>
                    <p className="modal-row-detail">{model.id} · {model.provider}{model.description ? ` · ${model.description}` : ''}</p>
                  </div>
                  {isCurrent ? <span className="modal-row-badge">Active</span> : null}
                </button>
              </li>
            );
          })}
        </ul>
        <footer className="modal-footer">
          <small>Enter to select · Esc to cancel · ↑/↓ to navigate</small>
        </footer>
      </div>
    </div>
  );
}

import { useEffect, useMemo, useRef, useState } from 'react';
import { X } from 'lucide-react';
import { listModels } from '../zero-api';

// Curated fallback used only when the live /providers/models endpoint is
// unreachable. Always provider/name format so the backend validator accepts.
const FALLBACK_MODELS: ModelEntry[] = [
  { id: 'cx/gpt-5.5', provider: '9router', label: 'GPT-5.5 (cx)', description: 'Default fast local-router model.' },
  { id: 'kr/claude-sonnet-4.5', provider: '9router', label: 'Claude Sonnet 4.5 (kr)', description: 'Strong reasoning via 9router.' },
  { id: 'openai/gpt-4o', provider: 'OpenAI', label: 'GPT-4o' },
  { id: 'openai/gpt-4o-mini', provider: 'OpenAI', label: 'GPT-4o mini' },
  { id: 'anthropic/claude-sonnet-4-5', provider: 'Anthropic', label: 'Claude Sonnet 4.5' },
];

type ModelEntry = {
  id: string;
  provider: string;
  label: string;
  description?: string;
};

function inferProvider(modelId: string): string {
  const slash = modelId.indexOf('/');
  if (slash <= 0) return 'unknown';
  const prefix = modelId.slice(0, slash).toLowerCase();
  switch (prefix) {
    case 'openai':
      return 'OpenAI';
    case 'anthropic':
      return 'Anthropic';
    case 'ollama':
      return 'Ollama';
    case 'kr':
    case 'cx':
      return '9router';
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

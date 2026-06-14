import { useEffect, useMemo, useRef, useState } from 'react';
import { X } from 'lucide-react';

// Curated model catalog. Keep IDs in `provider/name` form so the backend
// validator accepts them.
const MODELS: { id: string; provider: string; label: string; description?: string }[] = [
  { id: 'cx/gpt-5.5', provider: '9router', label: 'GPT-5.5 (cx)', description: 'Default fast local-router model.' },
  { id: 'kr/claude-sonnet-4.5', provider: '9router', label: 'Claude Sonnet 4.5 (kr)', description: 'Strong reasoning via 9router.' },
  { id: 'openai/gpt-4o', provider: 'OpenAI', label: 'GPT-4o', description: 'OpenAI flagship.' },
  { id: 'openai/gpt-4o-mini', provider: 'OpenAI', label: 'GPT-4o mini', description: 'Cheaper OpenAI tier.' },
  { id: 'anthropic/claude-sonnet-4-5', provider: 'Anthropic', label: 'Claude Sonnet 4.5', description: 'Anthropic flagship.' },
  { id: 'anthropic/claude-haiku-4-5', provider: 'Anthropic', label: 'Claude Haiku 4.5', description: 'Cheaper Anthropic tier.' },
  { id: 'ollama/llama3.1', provider: 'Ollama', label: 'Llama 3.1 (local)', description: 'Local model via Ollama.' },
];

export type ModelPickerModalProps = {
  open: boolean;
  currentModel: string;
  onClose: () => void;
  onSelect: (modelId: string) => void;
};

export function ModelPickerModal({ open, currentModel, onClose, onSelect }: ModelPickerModalProps) {
  const [query, setQuery] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (open) {
      setQuery('');
      setActiveIndex(0);
      // Defer focus so transition doesn't steal it.
      const id = window.setTimeout(() => inputRef.current?.focus(), 30);
      return () => window.clearTimeout(id);
    }
  }, [open]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return MODELS;
    return MODELS.filter(
      (m) => m.id.toLowerCase().includes(q) || m.label.toLowerCase().includes(q) || m.provider.toLowerCase().includes(q),
    );
  }, [query]);

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
            <h2>Select an AI model</h2>
          </div>
          <button aria-label="Close" className="modal-close" onClick={onClose} type="button">
            <X size={16} />
          </button>
        </header>
        <input
          className="modal-search"
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Filter by name, id, or provider…"
          ref={inputRef}
          value={query}
        />
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

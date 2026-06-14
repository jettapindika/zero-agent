import { useEffect, useRef, useState } from 'react';
import { getCurrentWebview } from '@tauri-apps/api/webview';

import { toDroppedFile, type DroppedFile } from './dropped-files';

type DropPhase = 'idle' | 'over' | 'drop';

export type UseFileDrop = {
  isDragOver: boolean;
  onDropped: (handler: (files: DroppedFile[]) => void) => void;
};

export function useFileDrop(): UseFileDrop {
  const [phase, setPhase] = useState<DropPhase>('idle');
  const dropHandler = useRef<(files: DroppedFile[]) => void>(() => {});

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (!('__TAURI_INTERNALS__' in window)) return;

    let unlisten: (() => void) | null = null;
    let cancelled = false;

    void (async () => {
      try {
        const webview = getCurrentWebview();
        const off = await webview.onDragDropEvent((event) => {
          const payload = event.payload;
          if (payload.type === 'enter' || payload.type === 'over') {
            setPhase('over');
            return;
          }
          if (payload.type === 'leave') {
            setPhase('idle');
            return;
          }
          if (payload.type === 'drop') {
            setPhase('idle');
            const paths = (payload.paths ?? []).filter((p) => typeof p === 'string' && p.length > 0);
            if (paths.length === 0) return;
            const files = paths.map(toDroppedFile);
            dropHandler.current(files);
          }
        });
        if (cancelled) {
          off();
          return;
        }
        unlisten = off;
      } catch (err) {
        console.warn('useFileDrop: failed to subscribe to drag events', err);
      }
    })();

    return () => {
      cancelled = true;
      unlisten?.();
    };
  }, []);

  return {
    isDragOver: phase === 'over',
    onDropped: (handler) => {
      dropHandler.current = handler;
    },
  };
}

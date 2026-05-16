type Listener = (event: Record<string, unknown>) => void;

export function createMockSession() {
  const listeners = new Set<Listener>();

  return {
    messages: [] as Array<{ id: string; role: string; content: string }>,
    isStreaming: false,
    promptCalls: [] as string[],
    aborted: false,
    cycledModel: false,
    subscribe(listener: Listener) {
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    emit(event: Record<string, unknown>) {
      for (const listener of listeners) {
        listener(event);
      }
    },
    async prompt(text: string) {
      this.promptCalls.push(text);
    },
    abort() {
      this.aborted = true;
    },
    cycleModel() {
      this.cycledModel = true;
    },
    dispose() {}
  };
}

export function createMockSessionManager() {
  return {
    async list() {
      return [
        { id: "session-1", title: "First session" },
        { id: "session-2", title: "Second session" }
      ];
    }
  };
}

import { render } from "ink";
import { PiSessionProvider } from "./providers/PiSessionProvider.js";
import { UIStateProvider } from "./providers/UIStateProvider.js";
import { App } from "./app.js";

if (!process.stdin.isTTY) {
  console.error("pi-opencode requires an interactive terminal (TTY). Run it directly in your terminal, not piped or backgrounded.");
  process.exit(1);
}

function Root() {
  return (
    <PiSessionProvider>
      <UIStateProvider>
        <App />
      </UIStateProvider>
    </PiSessionProvider>
  );
}

const { waitUntilExit } = render(<Root />, {
  exitOnCtrlC: false,
});

void waitUntilExit();

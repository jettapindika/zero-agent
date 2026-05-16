import { render } from "ink";
import { PiSessionProvider } from "./providers/PiSessionProvider.js";
import { UIStateProvider } from "./providers/UIStateProvider.js";
import { App } from "./app.js";

function Root() {
  return (
    <PiSessionProvider>
      <UIStateProvider>
        <App />
      </UIStateProvider>
    </PiSessionProvider>
  );
}

const apiKey = process.env.ANTHROPIC_API_KEY ?? process.env.PI_API_KEY;
if (!apiKey) {
  console.error("Error: No API key found. Set ANTHROPIC_API_KEY or PI_API_KEY environment variable.");
  process.exit(1);
}

const { waitUntilExit } = render(<Root />, {
  exitOnCtrlC: false,
});

void waitUntilExit();

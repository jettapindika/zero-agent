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

const { waitUntilExit } = render(<Root />, {
  exitOnCtrlC: false,
});

void waitUntilExit();

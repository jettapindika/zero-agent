# Zero

Terminal-first AI coding agent.

## Architecture

```txt
Terminal CLI/TUI → Go SDK → local HTTP/SSE core → providers/tools/storage
```

Zero runs locally. The terminal command is the primary interface; browser UI is legacy/optional and not required for normal use.

## Build

```bash
make build
```

This creates:

```txt
bin/zero
bin/zero-server
```

Install the command so you can run `zero` without `./bin/`:

```bash
make install
zero
```

By default this installs to `~/.local/bin`. If your shell cannot find `zero`, add `~/.local/bin` to `PATH`.

## First run

```bash
./bin/zero setup
./bin/zero start
./bin/zero status
./bin/zero
```

Stop the background server:

```bash
./bin/zero stop
```

## Commands

```bash
zero                 # open OpenCode-like terminal UI
zero -p "prompt"     # run one prompt
zero setup           # create ~/.zero and config
zero start           # start background server
zero stop            # stop background server
zero restart         # restart background server
zero status          # show PID + health
zero logs            # print ~/.zero/zero.log
zero sessions        # list sessions for current project
zero share           # create team session invite
zero join <invite>   # join team session
```

## TUI shortcuts

```txt
enter    send prompt
ctrl+j   send prompt (for terminals that map Enter to Ctrl+J)
ctrl+n   create new session
ctrl+p   command palette hint/reserved
ctrl+c   quit
```

The UI uses a responsive OpenCode-like layout: sidebar + chat on wide terminals, chat-focused mode on narrow terminals.

## TUI slash commands

```txt
/new                         create a new session
/clear, /reset               clear visible chat
/status, /info               show session/model/agent status
/history                     show message count for this session
/model [provider/model]      show or set model
/models                      show current model help
/agent [build|plan|explore]  show or set agent
/plan, /ask, /code           switch to plan/explore/build mode
/compact, /summarize         request context compaction
/editor, /edit               open $EDITOR for prompt drafting
/shortcuts, /keys            show keyboard shortcuts
/help                        show slash commands
/quit, /exit, /q             quit
```

## Provider

Default provider is 9router-compatible OpenAI API:

```txt
ZERO_ROUTER_BASE_URL=http://127.0.0.1:20128/v1
ZERO_ROUTER_API_KEY=sk_9router
```

Start 9router before sending prompts.

## Development

```bash
make test
make build
make run
```

## Structure

```txt
apps/cli          Go terminal command
apps/cli/tui      Bubble Tea terminal UI
packages/sdk-go   Go SDK used by CLI
services/core     Go backend server
apps/web          legacy/optional web UI
config/           config schema
```
# zero-agent

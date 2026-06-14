package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/zero-agent/core/pkg/dotenv"
	"github.com/zero-agent/core/pkg/identity"
	"github.com/zero-agent/core/pkg/server"
	sdk "github.com/zero-agent/sdk-go"
)

var serverAddr = "http://localhost:8910"

func newSDKClient(clientID string) *sdk.Client {
	return sdk.NewClient(serverAddr, sdk.Options{ClientID: clientID})
}

// defaultModel returns the model id used for new sessions when the user has
// not picked one. Override at runtime via the ZERO_DEFAULT_MODEL env var, or
// pick a different one in the desktop app's Model Picker. The compiled-in
// fallback is a small, low-cost OpenAI model so a stock checkout works
// against the public OpenAI API the moment a user supplies an API key.
func defaultModel() string {
	if v := os.Getenv("ZERO_DEFAULT_MODEL"); v != "" {
		return v
	}
	return "gpt-4o-mini"
}

var rootCmd = &cobra.Command{
	Use:   "zero [path]",
	Short: "AI coding agent",
	Long: "Zero — a production-grade terminal-first AI coding agent.\n\n" +
		"Run `zero .` (or `zero <path>`) in a folder to open the desktop app\n" +
		"rooted at that folder. With no args, opens the line-mode REPL.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && looksLikePath(args[0]) {
			return openDesktopAt(args[0])
		}
		prompt, _ := cmd.Flags().GetString("prompt")
		if prompt != "" {
			return runNonInteractive(prompt)
		}
		return runInteractive(cmd)
	},
}

func openDesktopAt(arg string) error {
	abs, err := resolveProjectDir(arg)
	if err != nil {
		return err
	}
	if err := writePendingProject(abs); err != nil {
		return fmt.Errorf("write pending-project file: %w", err)
	}
	if err := launchDesktop(); err != nil {
		return fmt.Errorf("launch desktop: %w", err)
	}
	fmt.Printf("Opening Zero desktop in %s\n", abs)
	return nil
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the backend server",
	RunE: func(cmd *cobra.Command, args []string) error {
		dotenv.Load()
		cfg := server.DefaultConfig()
		return server.Start(cfg)
	},
}

var serveDaemonCmd = &cobra.Command{
	Use:    "serve-daemon",
	Short:  "Start the backend server for zero start",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		dotenv.Load()
		cfg := server.DefaultConfig()
		return server.Start(cfg)
	},
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Initialize Zero local files",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureZeroDir(); err != nil {
			return err
		}
		configPath := filepath.Join(zeroDir(), "config.json")
		configCreated := false
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
				return err
			}
			configCreated = true
		}

		envDir, envPath := userEnvPath()
		envCreated := false
		if envPath != "" {
			if _, err := os.Stat(envPath); os.IsNotExist(err) {
				_ = os.MkdirAll(envDir, 0o700)
				if err := os.WriteFile(envPath, []byte(starterEnvFile), 0o600); err == nil {
					envCreated = true
				}
			}
		}

		fmt.Println("Zero setup complete")
		fmt.Println()
		fmt.Printf("  dir:     %s\n", zeroDir())
		fmt.Printf("  config:  %s%s\n", configPath, statusTag(configCreated))
		if envPath != "" {
			fmt.Printf("  env:     %s%s\n", envPath, statusTag(envCreated))
		}
		fmt.Printf("  pid:     %s\n", pidPath())
		fmt.Printf("  log:     %s\n", logPath())
		fmt.Println()
		fmt.Println("Required for the agent to actually do anything:")
		fmt.Println("  ZERO_ROUTER_BASE_URL  (default: https://api.openai.com/v1)")
		fmt.Println("  ZERO_ROUTER_API_KEY   (your provider's key — REQUIRED)")
		fmt.Println()
		fmt.Println("Pick a deployment mode (both work out of the box):")
		fmt.Println()
		fmt.Println("  A. Single-user (default)")
		fmt.Println("     Leave ZERO_AUTH_ENABLED unset. No login screen, anonymous,")
		fmt.Println("     all data in local SQLite at ~/.zero/zero.db.")
		fmt.Println()
		fmt.Println("  B. Multi-user — anyone with a Google account can sign in")
		fmt.Println("     Set ZERO_AUTH_ENABLED=true plus GOOGLE_CLIENT_ID/SECRET +")
		fmt.Println("     SESSION_SECRET. The daemon auto-creates users on first")
		fmt.Println("     login. Set ZERO_SUPABASE_DB_URL to share users across hosts.")
		fmt.Println()
		if envPath != "" && envCreated {
			fmt.Printf("Edit %s and set ZERO_ROUTER_API_KEY, then run `zero start`.\n", envPath)
		} else {
			fmt.Println("Set the env vars in your shell rc or ~/.config/zero/.env, then `zero start`.")
		}
		return nil
	},
}

func userEnvPath() (string, string) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", ""
	}
	dir := filepath.Join(home, ".config", "zero")
	return dir, filepath.Join(dir, ".env")
}

func statusTag(created bool) string {
	if created {
		return "  (created)"
	}
	return "  (already exists, kept as-is)"
}

const starterEnvFile = `# Zero local environment. This file is gitignored — never commit it.
# Lines starting with # are ignored.

# ----------------------------------------------------------------------------
# Provider — Bring Your Own Model (REQUIRED for the agent to work)
# Any OpenAI-compatible /v1 endpoint: OpenAI, OpenRouter, LiteLLM, Ollama,
# vLLM, llama.cpp, LM Studio, etc.
# ----------------------------------------------------------------------------
ZERO_ROUTER_BASE_URL=https://api.openai.com/v1
ZERO_ROUTER_API_KEY=

# Optional: pin a default model id. Otherwise Zero falls back to gpt-4o-mini.
# ZERO_DEFAULT_MODEL=gpt-4o-mini

# ----------------------------------------------------------------------------
# Multi-user mode (Google sign-in). Off by default → single-user, no login.
# Turn on by setting ZERO_AUTH_ENABLED=true plus the four Google fields below.
# When enabled, ANY Google account can sign in — the daemon upserts a row in
# the users table on first login (role: "user"). DEV_EMAILS is for elevated
# role assignment only, not for restricting who can sign in.
# ----------------------------------------------------------------------------
# ZERO_AUTH_ENABLED=true
# GOOGLE_CLIENT_ID=
# GOOGLE_CLIENT_SECRET=
# GOOGLE_CALLBACK_URL=http://127.0.0.1:8910/auth/google/callback
# SESSION_SECRET=
# DEV_EMAILS=                         # comma-separated, optional

# ----------------------------------------------------------------------------
# Optional: Supabase Postgres backs the multi-user auth tables. When set, the
# users + auth_sessions tables live in Supabase so the same identity follows
# users across multiple Zero hosts. When unset, those tables live in local
# SQLite — fine for a single-host multi-user deployment, one row per machine.
# ----------------------------------------------------------------------------
# ZERO_SUPABASE_DB_URL=postgresql://postgres:PASSWORD@db.<project-ref>.supabase.co:5432/postgres
`

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Zero server in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		return startDaemon(cmd.Context())
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Zero background server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return stopDaemon()
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart Zero background server",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = stopDaemon()
		return startDaemon(cmd.Context())
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Zero server status",
	RunE: func(cmd *cobra.Command, args []string) error {
		pid, err := readPID()
		if err != nil || !processRunning(pid) {
			fmt.Println("Zero server: stopped")
			return nil
		}
		fmt.Printf("Zero server: running (pid %d)\n", pid)
		if err := serverHealthy(cmd.Context()); err != nil {
			fmt.Printf("Health: failed (%v)\n", err)
			return nil
		}
		fmt.Println("Health: ok")
		return nil
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Print Zero server log",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, err := os.Open(logPath())
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(os.Stdout, file)
		return err
	},
}

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Create a team session and get an invite command",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		id, err := identity.Load()
		if err != nil {
			return fmt.Errorf("load identity: %w", err)
		}
		client := newSDKClient(id.ClientID)

		cwd, _ := os.Getwd()
		projectName := filepath.Base(cwd)
		project, err := client.EnsureProject(ctx, cwd, projectName)
		if err != nil {
			return fmt.Errorf("server unreachable: %w", err)
		}
		if project.ID == "" {
			return fmt.Errorf("failed to ensure project")
		}
		result, err := client.CreateCollabRoom(ctx, sdk.CreateRoomInput{
			ProjectID:        project.ID,
			Name:             projectName,
			DefaultRole:      "prompter",
			PromptReviewMode: "host_only",
			AutoRunQueue:     true,
		})
		if err != nil {
			return fmt.Errorf("failed to create room: %w", err)
		}

		inviteURL := fmt.Sprintf("zero://join/%s?token=%s", result.Room.ID, result.InviteToken)

		fmt.Println()
		fmt.Println("Team Session Created")
		fmt.Println()
		fmt.Printf("  Project: %s\n", projectName)
		fmt.Printf("  Room:    %s\n", result.Room.ID)
		fmt.Printf("  Role:    Host\n")
		fmt.Printf("  Review:  %s\n", result.Room.PromptReviewMode)
		fmt.Println()
		fmt.Println("Send this invite link (clicking opens the desktop app on Windows, macOS, and Linux):")
		fmt.Println()
		fmt.Printf("  %s\n", inviteURL)
		fmt.Println()
		fmt.Println("Other ways to join:")
		fmt.Printf("  CLI:           zero join %s\n", inviteURL)
		fmt.Printf("  CLI (flags):   zero join --room %s --token %s\n", result.Room.ID, result.InviteToken)
		fmt.Printf("  Desktop:       paste the link into Share folder \u2192 Join a room\n")
		fmt.Println()

		return nil
	},
}

var joinCmd = &cobra.Command{
	Use:   "join [invite-url]",
	Short: "Join a team session",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		id, err := identity.Load()
		if err != nil {
			return fmt.Errorf("load identity: %w", err)
		}
		client := newSDKClient(id.ClientID)

		roomID, _ := cmd.Flags().GetString("room")
		token, _ := cmd.Flags().GetString("token")

		if len(args) == 1 {
			r, t := parseInviteURL(args[0])
			if r != "" {
				roomID = r
			}
			if t != "" {
				token = t
			}
		}

		if roomID == "" || token == "" {
			return fmt.Errorf("provide invite URL or --room and --token flags")
		}

		result, err := client.JoinCollabRoom(ctx, roomID, token, id.DisplayName)
		if err != nil {
			if apiErr, ok := err.(*sdk.APIError); ok {
				if apiErr.StatusCode == 401 {
					return fmt.Errorf("invalid invite token")
				}
				if apiErr.StatusCode == 410 {
					return fmt.Errorf("room has been revoked")
				}
			}
			return fmt.Errorf("join failed: %w", err)
		}

		fmt.Println()
		fmt.Printf("Joined team session as %s\n", result.Participant.DisplayName)
		fmt.Println()
		fmt.Printf("  Room: %s\n", result.Room.ID)
		fmt.Printf("  Role: %s\n", result.Participant.Role)
		fmt.Println()

		return nil
	},
}

var participantsCmd = &cobra.Command{
	Use:   "participants",
	Short: "List team session participants",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newSDKClient("")
		roomID, _ := cmd.Flags().GetString("room")
		if roomID == "" {
			return fmt.Errorf("--room required")
		}

		participants, err := client.ListParticipants(cmd.Context(), roomID)
		if err != nil {
			return fmt.Errorf("list participants failed: %w", err)
		}

		fmt.Println()
		fmt.Println("Participants")
		fmt.Println()
		for _, p := range participants {
			mark := "○"
			if p.Status == "online" {
				mark = "●"
			}
			fmt.Printf("  %s %-16s %s\n", mark, p.DisplayName, p.Role)
		}
		fmt.Println()
		return nil
	},
}

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "List queued prompts for a session",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newSDKClient("")
		roomID, _ := cmd.Flags().GetString("room")
		sessionID, _ := cmd.Flags().GetString("session")
		if roomID == "" || sessionID == "" {
			return fmt.Errorf("--room and --session required")
		}

		items, err := client.ListPromptQueue(cmd.Context(), roomID, sessionID)
		if err != nil {
			return fmt.Errorf("list queue failed: %w", err)
		}

		fmt.Println()
		fmt.Println("Queue")
		fmt.Println()
		if len(items) == 0 {
			fmt.Println("  empty")
			fmt.Println()
			return nil
		}
		for _, item := range items {
			preview := item.Content
			if len(preview) > 80 {
				preview = preview[:77] + "..."
			}
			fmt.Printf("  #%d %s\n", item.Position, item.ActorClientID)
			fmt.Printf("     %s\n", preview)
			fmt.Printf("     %s\n", item.Status)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.Flags().StringP("prompt", "p", "", "Run prompt non-interactively")
	rootCmd.Flags().BoolP("continue", "c", false, "Continue the last session")
	rootCmd.Flags().StringP("session", "s", "", "Session ID to continue")
	rootCmd.Flags().Bool("fork", false, "Fork session when continuing")
	rootCmd.Flags().StringP("model", "m", "", "Model to use (provider/model)")
	rootCmd.Flags().String("agent", "", "Agent to use")
	joinCmd.Flags().String("room", "", "Room ID")
	joinCmd.Flags().String("token", "", "Invite token")
	participantsCmd.Flags().String("room", "", "Room ID")
	queueCmd.Flags().String("room", "", "Room ID")
	queueCmd.Flags().String("session", "", "Session ID")

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(serveDaemonCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(shareCmd)
	rootCmd.AddCommand(joinCmd)
	rootCmd.AddCommand(participantsCmd)
	rootCmd.AddCommand(queueCmd)
	rootCmd.AddCommand(sessionsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(agentsCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseInviteURL(url string) (roomID, token string) {
	prefix := "zero://join/"
	if !strings.HasPrefix(url, prefix) {
		return "", ""
	}
	rest := url[len(prefix):]
	parts := strings.SplitN(rest, "?token=", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func zeroDir() string {
	return filepath.Join(os.Getenv("HOME"), ".zero")
}

func pidPath() string {
	return filepath.Join(zeroDir(), "zero.pid")
}

func logPath() string {
	return filepath.Join(zeroDir(), "zero.log")
}

func ensureZeroDir() error {
	return os.MkdirAll(zeroDir(), 0o700)
}

func readPID() (int, error) {
	data, err := os.ReadFile(pidPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func serverHealthy(ctx context.Context) error {
	return newSDKClient("").Health(ctx)
}

func removeStalePID() {
	pid, err := readPID()
	if err != nil || !processRunning(pid) {
		_ = os.Remove(pidPath())
	}
}

func startDaemon(ctx context.Context) error {
	if err := ensureZeroDir(); err != nil {
		return err
	}
	if pid, err := readPID(); err == nil && processRunning(pid) {
		fmt.Printf("Zero server already running (pid %d)\n", pid)
		return nil
	}
	removeStalePID()

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(exe, "serve-daemon")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(pidPath(), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o600); err != nil {
		_ = cmd.Process.Kill()
		return err
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := serverHealthy(ctx); err == nil {
			fmt.Printf("Zero server started (pid %d)\n", cmd.Process.Pid)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("zero server did not become healthy within 5s; see %s", logPath())
}

func stopDaemon() error {
	pid, err := readPID()
	if err != nil {
		fmt.Println("Zero server already stopped")
		return nil
	}
	if !processRunning(pid) {
		_ = os.Remove(pidPath())
		fmt.Println("Zero server already stopped")
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	for i := 0; i < 30; i++ {
		if !processRunning(pid) {
			_ = os.Remove(pidPath())
			fmt.Println("Zero server stopped")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("zero server did not stop within 3s")
}

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List sessions for current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := identity.Load()
		if err != nil {
			return err
		}
		client := newSDKClient(id.ClientID)

		cwd, _ := os.Getwd()
		projectName := filepath.Base(cwd)
		project, err := client.EnsureProject(cmd.Context(), cwd, projectName)
		if err != nil {
			return fmt.Errorf("server unreachable: %w", err)
		}
		if project.ID == "" {
			return fmt.Errorf("failed to ensure project")
		}

		sessions, err := client.ListSessions(cmd.Context(), project.ID)
		if err != nil {
			return fmt.Errorf("list sessions: %w", err)
		}

		fmt.Println()
		fmt.Println("Sessions")
		fmt.Println()
		if len(sessions) == 0 {
			fmt.Println("  none")
		}
		for _, s := range sessions {
			fmt.Printf("  %s  %s  (%s / %s)\n", s.ID[:8], s.Title, s.Agent, s.Model)
		}
		fmt.Println()
		return nil
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := newSDKClient("").Health(cmd.Context()); err != nil {
			return fmt.Errorf("server unreachable: %w", err)
		}

		configPath := filepath.Join(os.Getenv("HOME"), ".zero", "config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Printf("No config file at %s\n", configPath)
			fmt.Println("Using defaults.")
			return nil
		}
		fmt.Println(string(data))
		return nil
	},
}

var exportCmd = &cobra.Command{
	Use:   "export [session-id]",
	Short: "Export session to Markdown",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID := args[0]
		messages, err := newSDKClient("").GetSessionMessages(cmd.Context(), sessionID)
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}

		fmt.Printf("# Session %s\n\n", sessionID)
		for _, msg := range messages {
			fmt.Printf("## %s\n\n", msg.Role)
			for _, part := range msg.Parts {
				if part.Text != nil {
					fmt.Println(*part.Text)
				} else {
					fmt.Printf("[%s]\n", part.Type)
				}
			}
			fmt.Println()
		}
		return nil
	},
}

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available models from provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		routerURL := os.Getenv("ZERO_ROUTER_BASE_URL")
		if routerURL == "" {
			routerURL = "https://api.openai.com/v1"
		}
		routerClient := sdk.NewClient(routerURL, sdk.Options{})
		resp, err := routerClient.ListModels(cmd.Context())
		if err != nil {
			return fmt.Errorf("list models from %s: %w (set ZERO_ROUTER_BASE_URL / ZERO_ROUTER_API_KEY to point at your provider)", routerURL, err)
		}
		fmt.Println()
		fmt.Println("Models")
		fmt.Println()
		for _, m := range resp {
			fmt.Printf("  %s\n", m)
		}
		fmt.Println()
		return nil
	},
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Show provider auth configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := filepath.Join(zeroDir(), "auth.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Printf("No auth file at %s\n", configPath)
			fmt.Println()
			fmt.Println("Zero reads its provider URL + API key from environment variables.")
			fmt.Println("Any OpenAI-compatible endpoint works (OpenAI, OpenRouter, LiteLLM, Ollama, vLLM, llama.cpp, ...).")
			fmt.Println()
			fmt.Println("  ZERO_ROUTER_BASE_URL  (default: https://api.openai.com/v1)")
			fmt.Println("  ZERO_ROUTER_API_KEY")
			fmt.Println()
			fmt.Println("Put them in ~/.config/zero/.env or your shell rc, then restart `zero start`.")
			return nil
		}
		fmt.Println(string(data))
		return nil
	},
}

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "List available agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		fmt.Println("Agents")
		fmt.Println()
		fmt.Printf("  %-12s %-20s %s\n", "NAME", "MODEL", "MODE")
		fmt.Printf("  %-12s %-20s %s\n", "build", defaultModel(), "read/write")
		fmt.Printf("  %-12s %-20s %s\n", "plan", defaultModel(), "read-only")
		fmt.Printf("  %-12s %-20s %s\n", "explore", defaultModel(), "read-only")
		fmt.Println()
		return nil
	},
}

func startEmbeddedServer() error {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:8910", 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return nil
	}
	go func() {
		cfg := server.DefaultConfig()
		server.Start(cfg)
	}()
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		conn, err := net.DialTimeout("tcp", "127.0.0.1:8910", 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
	}
	return fmt.Errorf("server failed to start within 3s")
}

func ensureSession() (string, error) {
	session, err := createTerminalSession(false)
	if err != nil {
		return "", err
	}
	return session.ID, nil
}

func createTerminalSession(alwaysNew bool) (sdk.Session, error) {
	ctx := context.Background()
	client := newSDKClient("")
	cwd, _ := os.Getwd()
	projectName := filepath.Base(cwd)

	project, err := client.EnsureProject(ctx, cwd, projectName)
	if err != nil {
		return sdk.Session{}, err
	}
	sessions, err := client.ListSessions(ctx, project.ID)
	if err != nil {
		return sdk.Session{}, err
	}
	if !alwaysNew && len(sessions) > 0 {
		for _, session := range sessions {
			if session.Model == defaultModel() {
				return session, nil
			}
		}
	}

	return client.CreateSession(ctx, sdk.CreateSessionInput{ProjectID: project.ID, Title: "terminal", Model: defaultModel(), Agent: "build"})
}

func sendPrompt(sessionID, prompt string) (string, error) {
	return sendPromptWithContext(context.Background(), sessionID, prompt)
}

func sendPromptWithContext(ctx context.Context, sessionID, prompt string) (string, error) {
	client := newSDKClient("")
	if err := client.SendMessage(ctx, sessionID, sdk.SendMessageInput{Role: "user", Text: prompt}); err != nil {
		return "", err
	}
	if err := client.RunSession(ctx, sessionID); err != nil {
		return "", err
	}
	messages, err := client.GetSessionMessages(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		if last.Role == "assistant" {
			var out strings.Builder
			for _, part := range last.Parts {
				if part.Text != nil {
					out.WriteString(*part.Text)
				}
			}
			return out.String(), nil
		}
	}
	return "", nil
}

func sendAndStream(sessionID, prompt string) error {
	answer, err := sendPrompt(sessionID, prompt)
	if err != nil {
		return err
	}
	if answer != "" {
		fmt.Println(answer)
	}
	return nil
}

func runInteractive(cmd *cobra.Command) error {
	if err := startEmbeddedServer(); err != nil {
		return err
	}

	continueFlag, _ := cmd.Flags().GetBool("continue")
	sessionFlag, _ := cmd.Flags().GetString("session")

	var session sdk.Session
	var err error

	switch {
	case sessionFlag != "":
		client := newSDKClient("")
		sessions, e := client.ListSessions(context.Background(), "")
		if e != nil {
			return e
		}
		for _, s := range sessions {
			if s.ID == sessionFlag || (len(s.ID) >= 8 && s.ID[:8] == sessionFlag) {
				session = s
				break
			}
		}
		if session.ID == "" {
			return fmt.Errorf("session %q not found", sessionFlag)
		}
	case continueFlag:
		session, err = createTerminalSession(false)
		if err != nil {
			return fmt.Errorf("session setup: %w", err)
		}
	default:
		session, err = createTerminalSession(false)
		if err != nil {
			return fmt.Errorf("session setup: %w", err)
		}
	}

	return runLineInteractive(session.ID)
}

func runLineInteractive(sessionID string) error {
	fmt.Println("Zero — AI coding agent")
	fmt.Printf("Session: %s\n", sessionID[:8])
	fmt.Println("Type your prompt. Ctrl+D to exit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\033[36m›\033[0m ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/quit" || line == "/exit" || line == "\x03" {
			break
		}
		if line == "/new" {
			session, err := createTerminalSession(true)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				continue
			}
			sessionID = session.ID
			fmt.Printf("New session: %s\n", sessionID[:8])
			continue
		}
		if err := sendAndStream(sessionID, line); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		fmt.Println()
	}
	fmt.Println("\nbye.")
	return scanner.Err()
}

func runNonInteractive(prompt string) error {
	if err := startEmbeddedServer(); err != nil {
		return err
	}
	sessionID, err := ensureSession()
	if err != nil {
		return fmt.Errorf("session setup: %w", err)
	}
	return sendAndStream(sessionID, prompt)
}

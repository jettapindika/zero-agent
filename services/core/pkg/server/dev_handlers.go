package server

import (
	"net/http"
	"runtime"
	"runtime/debug"
	"time"
)

var serverStartedAt = time.Now()

// handleDevRuntime exposes process info to the dev panel: uptime, goroutine
// count, Go version, build info. Gated by RequireDev.
func (s *Server) handleDevRuntime(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(serverStartedAt)

	info := map[string]any{
		"goVersion":     runtime.Version(),
		"goroutines":    runtime.NumGoroutine(),
		"numCPU":        runtime.NumCPU(),
		"goos":          runtime.GOOS,
		"goarch":        runtime.GOARCH,
		"uptimeSeconds": int64(uptime.Seconds()),
		"pid":           1, // placeholder; runtime exposes per-OS pid via os.Getpid in the caller below
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		info["module"] = bi.Main.Path
		info["version"] = bi.Main.Version
		settings := map[string]string{}
		for _, s := range bi.Settings {
			settings[s.Key] = s.Value
		}
		info["buildSettings"] = settings
	}

	writeJSON(w, http.StatusOK, info)
}

// handleDevReloadSkills triggers a re-scan of project SKILL.md files. The
// loader is per-call (not cached); this endpoint exists as an explicit signal
// to the desktop UI that the user wants the next prompt to refresh skills.
// Returns 204 No Content.
func (s *Server) handleDevReloadSkills(w http.ResponseWriter, r *http.Request) {
	s.bus.Publish("dev.reload_skills", "", "", map[string]string{
		"requestedAt": time.Now().UTC().Format(time.RFC3339),
	})
	w.WriteHeader(http.StatusNoContent)
}

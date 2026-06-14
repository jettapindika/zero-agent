package main

import (
	"log/slog"
	"os"

	"github.com/zero-agent/core/pkg/server"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// Pull in `.env` style files before reading config. Existing shell exports
	// always win — see loadDotEnv for the search order.
	loadDotEnv()

	cfg := server.DefaultConfig()
	if err := server.Start(cfg); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

package main

import (
	"log/slog"
	"os"

	"github.com/zero-agent/core/pkg/dotenv"
	"github.com/zero-agent/core/pkg/server"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	dotenv.Load()

	cfg := server.DefaultConfig()
	if err := server.Start(cfg); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

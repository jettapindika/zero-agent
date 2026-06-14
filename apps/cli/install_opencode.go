package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var installOpencodeCmd = &cobra.Command{
	Use:   "install-opencode",
	Short: "Port ~/.config/opencode/ into ~/.config/zero/",
	Long: `Scans the local OpenCode config directory and writes Zero equivalents.

What gets ported:
  ~/.config/opencode/agents.json     -> ~/.config/zero/agents.json
  ~/.config/opencode/AGENTS.md       -> ~/.config/zero/AGENTS.md
  ~/.config/opencode/opencode.json   -> ~/.config/zero/zero.json   (orchestration)
  ~/.config/opencode/mcp.json        -> ~/.config/zero/mcp.json
  ~/.config/opencode/lsp.json        -> ~/.config/zero/lsp.json
  ~/.config/opencode/profiles/*.json -> ~/.config/zero/profiles/*.json
  ~/.config/opencode/prompts/*.txt   -> ~/.config/zero/prompts/*.txt
  ~/.config/opencode/skills/**       -> ~/.config/zero/skills/**

Existing files in the destination are NOT overwritten unless --force is set.
The source is left untouched.`,
	RunE: runInstallOpencode,
}

func init() {
	installOpencodeCmd.Flags().Bool("force", false, "Overwrite existing destination files")
	installOpencodeCmd.Flags().Bool("dry-run", false, "Print what would happen without writing")
	rootCmd.AddCommand(installOpencodeCmd)
}

type installPlanEntry struct {
	src  string
	dst  string
	mode fs.FileMode
}

func runInstallOpencode(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	src := filepath.Join(home, ".config", "opencode")
	dst := filepath.Join(home, ".config", "zero")

	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("source %s does not exist; install OpenCode first or skip this step", src)
	}

	plan, err := buildInstallPlan(src, dst)
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		fmt.Println("Nothing to install — source is empty.")
		return nil
	}

	if dryRun {
		fmt.Printf("Would copy %d entries from %s to %s:\n", len(plan), src, dst)
		for _, entry := range plan {
			fmt.Printf("  %s\n", entry.dst)
		}
		return nil
	}

	written := 0
	skipped := 0
	for _, entry := range plan {
		if !force {
			if _, err := os.Stat(entry.dst); err == nil {
				skipped++
				continue
			}
		}
		if err := os.MkdirAll(filepath.Dir(entry.dst), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(entry.dst), err)
		}
		data, err := os.ReadFile(entry.src)
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.src, err)
		}
		// Rename opencode.json -> zero.json so we don't shadow OpenCode's config.
		dstPath := entry.dst
		if filepath.Base(dstPath) == "opencode.json" {
			dstPath = filepath.Join(filepath.Dir(dstPath), "zero.json")
		}
		if err := os.WriteFile(dstPath, data, entry.mode); err != nil {
			return fmt.Errorf("write %s: %w", dstPath, err)
		}
		written++
	}
	fmt.Printf("Installed %d files into %s (skipped %d existing). Use --force to overwrite.\n", written, dst, skipped)
	if !force && skipped > 0 {
		fmt.Println("Tip: re-run with --force if you want to refresh existing files.")
	}
	return nil
}

func buildInstallPlan(src, dst string) ([]installPlanEntry, error) {
	plan := make([]installPlanEntry, 0, 64)
	walkErr := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort
		}
		base := d.Name()
		if d.IsDir() {
			// Skip noisy/heavy directories that aren't config.
			if base == "node_modules" || base == ".git" || base == "cli" {
				return fs.SkipDir
			}
			return nil
		}
		// Skip CLI bookkeeping files OpenCode keeps next to its config.
		if base == "cli-payload.txt" || base == "fallback.json" || base == "package.json" || base == "package-lock.json" || base == "tui.json" {
			return nil
		}
		// Only port known config formats.
		if !strings.HasSuffix(base, ".json") && !strings.HasSuffix(base, ".md") && !strings.HasSuffix(base, ".txt") {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		plan = append(plan, installPlanEntry{src: path, dst: filepath.Join(dst, rel), mode: info.Mode().Perm()})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return plan, nil
}

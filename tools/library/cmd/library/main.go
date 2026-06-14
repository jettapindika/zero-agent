package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zero/tools/library/internal/lib"
)

func usage() {
	fmt.Fprint(os.Stderr, `library - agent knowledge library

Usage:
  library <command> [flags]

Commands:
  init                    Create library/ directory layout
  add                     Capture a new entry (flags or stdin JSON)
  list                    List active entries (sorted by score)
  show <id>               Show one entry as JSON
  delete <id>             Archive an entry (entries are never hard-deleted)
  restore <id>            Restore an archived entry
  stats                   Print counts and ranking summary
  inject                  Print the markdown block for prompt injection
  use <id>                Mark entry as used (use --helped to count as a hit)
  reindex                 Rebuild index.json, tags.json, LIBRARY.md
  auto-archive            Archive low-scoring, sufficiently-used entries
  capture-correction      Capture a user-correction entry from stdin or flags

Global flags:
  --root <dir>            Library root (default: ./library)
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	root := defaultRoot()
	cmd := os.Args[1]
	args := os.Args[2:]
	args, root = stripRoot(args, root)

	store := lib.New(root)

	switch cmd {
	case "-h", "--help", "help":
		usage()
	case "init":
		must(store.Init())
		fmt.Println("initialized:", root)
	case "add":
		runAdd(store, args, lib.SourceManual)
	case "capture-correction":
		runAdd(store, args, lib.SourceUserCorrection)
	case "list":
		runList(store, args)
	case "show":
		runShow(store, args)
	case "delete":
		runDelete(store, args)
	case "restore":
		runRestore(store, args)
	case "stats":
		runStats(store)
	case "inject":
		runInject(store)
	case "use":
		runUse(store, args)
	case "reindex":
		must(store.Reindex())
		fmt.Println("reindexed")
	case "auto-archive":
		runAutoArchive(store, args)
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", cmd)
		usage()
		os.Exit(2)
	}
}

func defaultRoot() string {
	if v := os.Getenv("LIBRARY_ROOT"); v != "" {
		return v
	}
	wd, err := os.Getwd()
	if err != nil {
		return "library"
	}
	return filepath.Join(wd, "library")
}

func stripRoot(args []string, root string) ([]string, string) {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--root" && i+1 < len(args):
			root = args[i+1]
			i++
		case strings.HasPrefix(a, "--root="):
			root = strings.TrimPrefix(a, "--root=")
		default:
			out = append(out, a)
		}
	}
	return out, root
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runAdd(store *lib.Store, args []string, defaultSource lib.Source) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	typeStr := fs.String("type", "", "entry type: mistake|insight|pattern|convention|fix")
	title := fs.String("title", "", "short title")
	body := fs.String("body", "", "full body (use --stdin to read body from stdin)")
	tags := fs.String("tags", "", "comma-separated tags")
	files := fs.String("files", "", "comma-separated file paths the entry refers to")
	source := fs.String("source", string(defaultSource), "source: agent|user-correction|manual|retry")
	confidence := fs.Float64("confidence", 0.7, "confidence 0..1")
	jsonStdin := fs.Bool("json", false, "read full entry as JSON from stdin")
	bodyStdin := fs.Bool("stdin", false, "read body text from stdin")
	must(fs.Parse(args))

	var e lib.Entry
	if *jsonStdin {
		dec := json.NewDecoder(os.Stdin)
		must(dec.Decode(&e))
	} else {
		e = lib.Entry{
			Type:       lib.EntryType(*typeStr),
			Title:      *title,
			Body:       *body,
			Tags:       splitCSV(*tags),
			FilePaths:  splitCSV(*files),
			Source:     lib.Source(*source),
			Confidence: *confidence,
		}
		if *bodyStdin {
			b, err := io.ReadAll(os.Stdin)
			must(err)
			e.Body = strings.TrimSpace(string(b))
		}
	}

	if e.Type == "" {
		fmt.Fprintln(os.Stderr, "error: --type required")
		os.Exit(2)
	}

	must(store.Put(&e))
	fmt.Println(e.ID)
}

func runList(store *lib.Store, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	includeArchived := fs.Bool("all", false, "include archived entries")
	jsonOut := fs.Bool("json", false, "JSON output")
	tagFilter := fs.String("tag", "", "filter by tag")
	typeFilter := fs.String("type", "", "filter by type")
	limit := fs.Int("limit", 0, "max rows (0 = no limit)")
	must(fs.Parse(args))

	entries, err := store.List(*includeArchived)
	must(err)

	if *tagFilter != "" {
		entries = filter(entries, func(e lib.Entry) bool {
			for _, t := range e.Tags {
				if t == strings.ToLower(*tagFilter) {
					return true
				}
			}
			return false
		})
	}
	if *typeFilter != "" {
		entries = filter(entries, func(e lib.Entry) bool { return string(e.Type) == *typeFilter })
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Score() > entries[j].Score() })
	if *limit > 0 && len(entries) > *limit {
		entries = entries[:*limit]
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		must(enc.Encode(entries))
		return
	}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	if len(entries) == 0 {
		fmt.Fprintln(w, "(no entries)")
		return
	}
	fmt.Fprintf(w, "%-32s  %-10s  %5s  %5s  %5s  %s\n", "ID", "TYPE", "SCORE", "USE", "HIT", "TITLE")
	for _, e := range entries {
		marker := ""
		if e.Archived {
			marker = " [archived]"
		}
		fmt.Fprintf(w, "%-32s  %-10s  %.2f   %5d  %5d  %s%s\n",
			e.ID, e.Type, e.Score(), e.UseCount, e.HitCount, e.Title, marker)
	}
}

func runShow(store *lib.Store, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "show requires <id>")
		os.Exit(2)
	}
	e, err := store.Get(args[0])
	must(err)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	must(enc.Encode(e))
}

func runDelete(store *lib.Store, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "delete requires <id>")
		os.Exit(2)
	}
	must(store.Archive(args[0]))
	fmt.Println("archived:", args[0])
}

func runRestore(store *lib.Store, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "restore requires <id>")
		os.Exit(2)
	}
	must(store.Restore(args[0]))
	fmt.Println("restored:", args[0])
}

func runStats(store *lib.Store) {
	active, err := store.List(false)
	must(err)
	archived, err := store.List(true)
	must(err)
	archivedOnly := 0
	for _, e := range archived {
		if e.Archived {
			archivedOnly++
		}
	}
	counts := map[lib.EntryType]int{}
	for _, e := range active {
		counts[e.Type]++
	}
	fmt.Printf("Total active:   %d\n", len(active))
	fmt.Printf("Total archived: %d\n", archivedOnly)
	for _, t := range lib.AllTypes() {
		fmt.Printf("  %-10s %d\n", t, counts[t])
	}
}

func runInject(store *lib.Store) {
	entries, err := store.Inject()
	must(err)
	fmt.Print(lib.RenderInjection(entries))
}

func runUse(store *lib.Store, args []string) {
	fs := flag.NewFlagSet("use", flag.ExitOnError)
	helped := fs.Bool("helped", false, "count this use as a hit")
	must(fs.Parse(args))
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "use requires <id>")
		os.Exit(2)
	}
	must(store.IncrementUse(fs.Arg(0), *helped))
	fmt.Println("ok")
}

func runAutoArchive(store *lib.Store, args []string) {
	fs := flag.NewFlagSet("auto-archive", flag.ExitOnError)
	minUse := fs.Int("min-use", 5, "minimum use count before considering archival")
	threshold := fs.Float64("threshold", 0.30, "archive if score below this")
	must(fs.Parse(args))
	ids, err := store.AutoArchive(*minUse, *threshold)
	must(err)
	if len(ids) == 0 {
		fmt.Println("no entries archived")
		return
	}
	for _, id := range ids {
		fmt.Println("archived:", id)
	}
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func filter(in []lib.Entry, ok func(lib.Entry) bool) []lib.Entry {
	out := make([]lib.Entry, 0, len(in))
	for _, e := range in {
		if ok(e) {
			out = append(out, e)
		}
	}
	return out
}

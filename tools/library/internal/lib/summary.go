package lib

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// WriteSummary regenerates LIBRARY.md from the given active entries.
func (s *Store) WriteSummary(active []Entry) error {
	counts := map[EntryType]int{}
	for _, e := range active {
		counts[e.Type]++
	}

	ranked := append([]Entry(nil), active...)
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].Score() > ranked[j].Score() })

	recent := append([]Entry(nil), active...)
	sort.Slice(recent, func(i, j int) bool { return recent[i].CreatedAt.After(recent[j].CreatedAt) })

	var b strings.Builder
	fmt.Fprintf(&b, "# Agent Library\n")
	fmt.Fprintf(&b, "Last updated: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "Total entries: %d (%d mistakes, %d insights, %d patterns, %d conventions, %d fixes)\n\n",
		len(active),
		counts[TypeMistake], counts[TypeInsight], counts[TypePattern], counts[TypeConvention], counts[TypeFix],
	)

	fmt.Fprintf(&b, "## Top Entries by Score\n")
	if len(ranked) == 0 {
		fmt.Fprintln(&b, "_no entries yet_")
	}
	topN := 10
	if len(ranked) < topN {
		topN = len(ranked)
	}
	for i := 0; i < topN; i++ {
		e := ranked[i]
		conf := int(e.Confidence * 100)
		fmt.Fprintf(&b, "%d. [%s] %s\n   confidence: %d%% · used: %d · helped: %d · score: %.2f\n",
			i+1, strings.ToUpper(string(e.Type)), e.Title, conf, e.UseCount, e.HitCount, e.Score(),
		)
	}

	fmt.Fprintf(&b, "\n## Recent Additions\n")
	if len(recent) == 0 {
		fmt.Fprintln(&b, "_no entries yet_")
	}
	recentN := 10
	if len(recent) < recentN {
		recentN = len(recent)
	}
	for i := 0; i < recentN; i++ {
		e := recent[i]
		fmt.Fprintf(&b, "- %s [%s] %s\n", e.CreatedAt.Format("2006-01-02"), strings.ToUpper(string(e.Type)), e.Title)
	}

	return os.WriteFile(s.summaryFile(), []byte(b.String()), 0o644)
}

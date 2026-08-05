package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/moul/workgraph/internal/index"
)

// cmdTimeline prints the recent activity feed derived from the event log.
func cmdTimeline(args []string) error {
	fs := flag.NewFlagSet("timeline", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	limit := fs.Int("limit", 30, "max rows to show")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	res, err := index.Build(ws, false)
	if err != nil {
		return err
	}
	rows := res.Timeline
	if *limit > 0 && len(rows) > *limit {
		rows = rows[:*limit]
	}
	if *jsonOut {
		return printJSON(rows)
	}
	if len(rows) == 0 {
		fmt.Println("no activity yet")
		return nil
	}
	tw := newTab()
	fmt.Fprintln(tw, "WHEN\tACTOR\tACTION\tOBJECT\tDETAIL")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", shortTime(r.At), shortWorker(r.Actor), r.Action, orDash(r.Object), timelineDetail(r))
	}
	return tw.Flush()
}

func timelineDetail(r index.TimelineLine) string {
	var parts []string
	if r.From != "" || r.To != "" {
		parts = append(parts, r.From+"→"+r.To)
	}
	if r.Status != "" {
		parts = append(parts, "["+r.Status+"]")
	}
	if r.Message != "" {
		parts = append(parts, r.Message)
	}
	return strings.Join(parts, " ")
}

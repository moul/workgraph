package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/moul/workgraph/internal/index"
)

// cmdHealth prints a qualitative, explainable health suggestion per project.
// Health is never a percentage — it is derived on demand (time-dependent) and
// never committed to an index.
func cmdHealth(args []string) error {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	g, err := ws.Load()
	if err != nil {
		return err
	}
	rows := index.Health(g, time.Now())
	if *jsonOut {
		return printJSON(rows)
	}
	if len(rows) == 0 {
		fmt.Println("no projects")
		return nil
	}
	tw := newTab()
	fmt.Fprintln(tw, "PROJECT\tSUGGESTED\tEXPLICIT\tWHY")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Title, r.Suggested, orDash(r.Explicit), strings.Join(r.Reasons, "; "))
	}
	return tw.Flush()
}

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/moul/workgraph/internal/index"
	"github.com/moul/workgraph/internal/webui"
)

func cmdUI(args []string) error {
	fs := flag.NewFlagSet("ui", flag.ExitOnError)
	_ = fs.Bool("static", false, "write a static HTML site instead of serving")
	serve := fs.Bool("serve", false, "serve the read-only UI over HTTP")
	addr := fs.String("addr", ":8081", "serve address (with --serve)")
	out := fs.String("out", "", "output directory for --static (default: <ws>/site)")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}

	if *serve {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Rebuild on each request so the read-only view stays fresh.
			res, err := index.Build(ws, false)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, webui.Render(res))
		})
		fmt.Printf("read-only UI on http://localhost%s\n", *addr)
		return http.ListenAndServe(*addr, nil)
	}

	res, err := index.Build(ws, false)
	if err != nil {
		return err
	}
	dir := *out
	if dir == "" {
		dir = filepath.Join(ws.Root, "site")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(webui.Render(res)), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote static site to %s/index.html\n", dir)
	return nil
}

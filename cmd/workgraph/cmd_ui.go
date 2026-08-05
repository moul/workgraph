package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moul/workgraph/internal/index"
	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
	"github.com/moul/workgraph/internal/webui"
)

func cmdUI(args []string) error {
	fs := flag.NewFlagSet("ui", flag.ExitOnError)
	_ = fs.Bool("static", false, "write a static HTML site instead of serving")
	serve := fs.Bool("serve", false, "serve the dashboard over HTTP")
	write := fs.Bool("write", false, "enable write actions (status changes); binds to localhost")
	addr := fs.String("addr", ":8081", "serve address (with --serve)")
	out := fs.String("out", "", "output directory for --static (default: <ws>/site)")
	var o globalOpts
	addGlobal(fs, &o) // -C, --actor, --no-push, --offline, ...
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}

	if *serve {
		return serveUI(ws, &o, *addr, *write)
	}

	// Static export is always read-only.
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
	nn, err := webui.ExportStatic(res, dir)
	if err != nil {
		return err
	}
	fmt.Printf("wrote static site to %s (%d files: index.html, data/*.json, assets/*.svg)\n", dir, nn)
	return nil
}

func serveUI(ws *store.Workspace, o *globalOpts, addr string, write bool) error {
	// The write UI mutates the repo, so it must not be exposed on all
	// interfaces: force localhost when a host wasn't given explicitly.
	if write && strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		res, err := index.Build(ws, false)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, webui.RenderServed(res, write))
	})

	// SSE stream: emit the workspace revision so browsers reload on change.
	mux.HandleFunc("/wg/stream", func(w http.ResponseWriter, r *http.Request) {
		streamRevision(w, r, ws.Root)
	})

	if write {
		mux.HandleFunc("/wg/status", func(w http.ResponseWriter, r *http.Request) {
			handleSetStatus(w, r, o)
		})
	}

	mode := "read-only"
	if write {
		mode = "WRITEABLE"
	}
	fmt.Printf("%s dashboard on http://%s\n", mode, addr)
	return http.ListenAndServe(addr, mux)
}

// streamRevision serves Server-Sent Events: it emits a cheap workspace
// "revision" fingerprint every second so the browser reloads when anything
// changes. SSE (not WebSocket) keeps this dependency-free.
func streamRevision(w http.ResponseWriter, r *http.Request, root string) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	last := ""
	for {
		rev := revision(root)
		if rev != last {
			fmt.Fprintf(w, "data: %s\n\n", rev)
			fl.Flush()
			last = rev
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// revision fingerprints the source planes (events + object files) by size and
// modtime, so any mutation changes it. Cheap and good enough to trigger a
// reload.
func revision(root string) string {
	var sum int64
	for _, sub := range []string{"events", "projects", "inbox", "workers"} {
		filepath.Walk(filepath.Join(root, sub), func(_ string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			sum += info.Size() + info.ModTime().UnixNano()
			return nil
		})
	}
	return fmt.Sprintf("%d", sum)
}

func handleSetStatus(w http.ResponseWriter, r *http.Request, o *globalOpts) {
	if r.Method != http.MethodPost {
		writeUIErr(w, 405, "POST only")
		return
	}
	var in struct{ ID, Status, Version string }
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeUIErr(w, 400, "bad JSON")
		return
	}
	e, err := engine(o)
	if err != nil {
		writeUIErr(w, 500, err.Error())
		return
	}
	if !e.WS.Ontology.Has("item_status", in.Status) {
		writeUIErr(w, 400, "unknown status "+in.Status)
		return
	}
	it, err := e.UpdateItem(in.ID, func(i *model.Item) { i.Status = in.Status }, in.Version, "ui: set status "+in.Status)
	if err != nil {
		// A version conflict is the expected optimistic-concurrency failure.
		code := 400
		if strings.Contains(err.Error(), "version conflict") {
			code = 409
		}
		writeUIErr(w, code, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": it.ID, "status": it.Status})
}

func writeUIErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

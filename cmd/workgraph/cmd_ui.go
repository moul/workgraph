package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/moul/workgraph/internal/index"
)

func cmdUI(args []string) error {
	fs := flag.NewFlagSet("ui", flag.ExitOnError)
	static := fs.Bool("static", false, "write a static HTML site instead of serving")
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
	res, err := index.Build(ws, false)
	if err != nil {
		return err
	}
	htmlDoc := renderUI(res)

	if *serve {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Rebuild on each request so the read-only view stays fresh.
			res, err := index.Build(ws, false)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, renderUI(res))
		})
		fmt.Printf("read-only UI on http://localhost%s\n", *addr)
		return http.ListenAndServe(*addr, nil)
	}

	dir := *out
	if dir == "" {
		dir = filepath.Join(ws.Root, "site")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if !*static {
		// default action is static generation
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(htmlDoc), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote static site to %s/index.html\n", dir)
	return nil
}

// renderUI produces a self-contained, dependency-free HTML page from the index.
func renderUI(res *index.Result) string {
	objs, _ := json.Marshal(res.Objects)
	att, _ := json.Marshal(res.Attention)
	runs, _ := json.Marshal(res.Runs)

	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width,initial-scale=1">`)
	b.WriteString(`<title>Workgraph</title><style>`)
	b.WriteString(`body{font-family:ui-monospace,Menlo,monospace;margin:1.5rem;color:#111;background:#fff}`)
	b.WriteString(`h1{font-size:1.3rem}table{border-collapse:collapse;width:100%;font-size:.85rem}`)
	b.WriteString(`th,td{text-align:left;padding:.3rem .5rem;border-bottom:1px solid #eee;vertical-align:top}`)
	b.WriteString(`th{cursor:pointer;position:sticky;top:0;background:#fafafa}`)
	b.WriteString(`.pill{padding:.05rem .4rem;border-radius:10px;background:#eee;font-size:.75rem}`)
	b.WriteString(`.high{color:#b00}.controls{margin:.5rem 0}input,select{font:inherit;padding:.2rem}`)
	b.WriteString(`@media(prefers-color-scheme:dark){body{background:#111;color:#ddd}th{background:#1a1a1a}th,td{border-color:#333}.pill{background:#333}}`)
	b.WriteString(`</style></head><body>`)
	b.WriteString(`<h1>Workgraph <span class="pill" id="count"></span></h1>`)
	b.WriteString(`<p><em>Read-only projection of indexes/*.jsonl. The Markdown files are the source of truth.</em></p>`)
	b.WriteString(`<div class="controls">`)
	b.WriteString(`filter <input id="q" placeholder="text...">`)
	b.WriteString(` status <select id="status"><option value="">all</option></select>`)
	b.WriteString(` type <select id="type"><option value="">all</option></select>`)
	b.WriteString(`</div>`)
	b.WriteString(`<h2>Attention</h2><table id="att"><thead><tr><th>id</th><th>severity</th><th>reason</th><th>summary</th></tr></thead><tbody></tbody></table>`)
	b.WriteString(`<h2>Objects</h2><table id="objs"><thead><tr><th data-k="id">id</th><th data-k="type">type</th><th data-k="status">status</th><th data-k="title">title</th><th data-k="target_repo">target</th></tr></thead><tbody></tbody></table>`)
	fmt.Fprintf(&b, `<script>const OBJS=%s,ATT=%s,RUNS=%s;`, objs, att, runs)
	b.WriteString(uiScript)
	b.WriteString(`</script></body></html>`)
	_ = html.EscapeString
	return b.String()
}

const uiScript = `
function esc(s){return (s||'').replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]))}
function fill(sel,vals){const el=document.getElementById(sel);[...new Set(vals.filter(Boolean))].sort().forEach(v=>{const o=document.createElement('option');o.value=v;o.textContent=v;el.appendChild(o)})}
fill('status',OBJS.map(o=>o.status));fill('type',OBJS.map(o=>o.type));
function short(r){if(!r)return '';const p=r.split('/');return (p[p.length-1]||r).replace(/\.git$/,'')}
function renderAtt(){const tb=document.querySelector('#att tbody');tb.innerHTML=ATT.map(a=>'<tr><td>'+esc(a.id)+'</td><td class="'+(a.severity==='high'?'high':'')+'">'+esc(a.severity)+'</td><td>'+esc(a.reason)+'</td><td>'+esc(a.summary)+'</td></tr>').join('')}
function renderObjs(){const q=document.getElementById('q').value.toLowerCase();const st=document.getElementById('status').value;const ty=document.getElementById('type').value;
const rows=OBJS.filter(o=>(!st||o.status===st)&&(!ty||o.type===ty)&&(!q||JSON.stringify(o).toLowerCase().includes(q)));
document.getElementById('count').textContent=rows.length+' / '+OBJS.length;
document.querySelector('#objs tbody').innerHTML=rows.map(o=>'<tr><td>'+esc(o.id)+'</td><td>'+esc(o.type)+'</td><td><span class="pill">'+esc(o.status)+'</span></td><td>'+esc(o.title)+'<br><small>'+esc(o.summary||'')+'</small></td><td>'+esc(short(o.target_repo))+'</td></tr>').join('')}
['q','status','type'].forEach(id=>document.getElementById(id).addEventListener('input',renderObjs));
document.querySelectorAll('#objs th[data-k]').forEach(th=>th.addEventListener('click',()=>{const k=th.dataset.k;OBJS.sort((a,b)=>String(a[k]).localeCompare(String(b[k])));renderObjs()}));
renderAtt();renderObjs();
`

// Package webui renders a self-contained, dependency-free read-only dashboard
// from a built index. The same HTML is served by `workgraph ui` and by the
// gateway at /ui. It is a projection: the Markdown files remain the source of
// truth, and this view never writes.
package webui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moul/workgraph/internal/index"
)

// jsonArray marshals v, coercing a nil slice's "null" to "[]" so the embedded
// client script can always treat the value as an array (ATT.length etc.).
func jsonArray(v any) []byte {
	b, _ := json.Marshal(v)
	if string(b) == "null" {
		return []byte("[]")
	}
	return b
}

// Render produces the static, read-only dashboard HTML (no server features).
// Used by `workgraph ui --static` and the gateway `/ui`.
func Render(res *index.Result) string { return render(res, false, false) }

// RenderServed produces the dashboard for the local `workgraph ui --serve`
// server: it subscribes to a live SSE stream and, when writeable, adds inline
// write controls that POST back.
func RenderServed(res *index.Result, writeable bool) string { return render(res, writeable, true) }

func render(res *index.Result, writeable, live bool) string {
	objs := jsonArray(res.Objects)
	att := jsonArray(res.Attention)
	runs := jsonArray(res.Runs)
	health := jsonArray(res.Health)
	timeline := jsonArray(res.Timeline)

	actionsHead := ""
	footer := "Read-only projection of <code>indexes/*.jsonl</code>. The Markdown files are the source of truth."
	if writeable {
		actionsHead = `<th>set status</th>`
		footer = "Writeable local UI. Changes go through the same core mutation path as the CLI (object version checked, stale writes refused) and are committed to Git."
	}

	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width,initial-scale=1">`)
	b.WriteString(`<title>Workgraph</title><style>`)
	b.WriteString(uiCSS)
	b.WriteString(`</style></head><body>`)
	b.WriteString(`<header><div class="brand">◆ Workgraph</div><div class="stats" id="stats"></div></header>`)
	b.WriteString(`<div id="toast"></div>`)
	b.WriteString(`<main>`)
	b.WriteString(`<section id="attn-section"><h2>Needs attention <span class="muted" id="attn-count"></span></h2><div class="cards" id="attn"></div></section>`)
	b.WriteString(`<section><h2>Projects</h2><div class="cards" id="projects"></div></section>`)
	b.WriteString(`<section><div class="row"><h2>Items</h2><div class="controls">`)
	b.WriteString(`<input id="q" placeholder="filter…"><select id="status"><option value="">status: all</option></select><select id="ty"><option value="">type: all</option></select></div></div>`)
	fmt.Fprintf(&b, `<table id="objs"><thead><tr><th data-k="status">status</th><th data-k="title">title</th><th data-k="priority">priority</th><th data-k="owner">owner</th><th data-k="target_repo">target</th>%s</tr></thead><tbody></tbody></table></section>`, actionsHead)
	b.WriteString(`<section id="runs-section"><h2>Recent runs</h2><table id="runs"><thead><tr><th>run</th><th>round</th><th>worker</th><th>status</th><th>result</th></tr></thead><tbody></tbody></table></section>`)
	b.WriteString(`<section id="tl-section"><h2>Timeline</h2><table id="tl"><thead><tr><th>when</th><th>actor</th><th>action</th><th>object</th><th>detail</th></tr></thead><tbody></tbody></table></section>`)
	b.WriteString(`</main>`)
	fmt.Fprintf(&b, `<footer>%s</footer>`, footer)
	fmt.Fprintf(&b, `<script>const OBJS=%s,ATT=%s,RUNS=%s,HEALTH=%s,TL=%s,WRITEABLE=%t,LIVE=%t;`, objs, att, runs, health, timeline, writeable, live)
	b.WriteString(uiScript)
	b.WriteString(`</script></body></html>`)
	return b.String()
}

const uiCSS = `
:root{--bg:#fbfbfa;--fg:#1a1a1a;--muted:#6b7280;--card:#fff;--line:#e6e6e3;--accent:#0a7d33}
@media(prefers-color-scheme:dark){:root{--bg:#0f1115;--fg:#e6e6e6;--muted:#9aa0aa;--card:#171a21;--line:#2a2f3a;--accent:#4ade80}}
*{box-sizing:border-box}
body{margin:0;font-family:ui-sans-serif,system-ui,-apple-system,Segoe UI,Roboto,sans-serif;background:var(--bg);color:var(--fg);line-height:1.5}
header{position:sticky;top:0;z-index:5;display:flex;align-items:center;justify-content:space-between;gap:1rem;flex-wrap:wrap;
  padding:.7rem 1.25rem;background:var(--card);border-bottom:1px solid var(--line)}
.brand{font-weight:700;letter-spacing:.02em}
.stats{display:flex;gap:.5rem;flex-wrap:wrap}
.stat{font-size:.8rem;padding:.2rem .55rem;border:1px solid var(--line);border-radius:999px;color:var(--muted)}
.stat b{color:var(--fg)}
main{max-width:64rem;margin:1.25rem auto;padding:0 1.25rem;display:flex;flex-direction:column;gap:1.75rem}
h2{font-size:1rem;margin:0 0 .6rem;font-weight:650}
.muted{color:var(--muted);font-weight:400}
.row{display:flex;align-items:baseline;justify-content:space-between;gap:1rem;flex-wrap:wrap}
.controls{display:flex;gap:.4rem;flex-wrap:wrap}
input,select{font:inherit;font-size:.85rem;padding:.3rem .5rem;background:var(--card);color:var(--fg);border:1px solid var(--line);border-radius:7px}
.cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(15rem,1fr));gap:.7rem}
.card{background:var(--card);border:1px solid var(--line);border-radius:11px;padding:.75rem .85rem}
.card .t{font-weight:600;font-size:.9rem}.card .s{color:var(--muted);font-size:.8rem;margin-top:.2rem}
.card.sev-high{border-left:3px solid #e5484d}.card.sev-medium{border-left:3px solid #f5a623}.card.sev-low{border-left:3px solid var(--muted)}
table{border-collapse:collapse;width:100%;font-size:.85rem;background:var(--card);border:1px solid var(--line);border-radius:11px;overflow:hidden}
th,td{text-align:left;padding:.5rem .7rem;border-bottom:1px solid var(--line);vertical-align:top}
th{cursor:pointer;font-weight:600;color:var(--muted);user-select:none;font-size:.78rem;text-transform:uppercase;letter-spacing:.03em}
tbody tr:last-child td{border-bottom:0}
.pill{display:inline-block;padding:.08rem .5rem;border-radius:999px;font-size:.72rem;font-weight:600;border:1px solid transparent}
.st-ready{background:#e7f6ec;color:#0a7d33}.st-in_progress{background:#e7f0fb;color:#1d5fd1}
.st-blocked{background:#fdeaea;color:#c22}.st-review{background:#fdf4e3;color:#b7791f}
.st-done{background:#eef0f2;color:#556}.st-triage,.st-inbox{background:#f1f0ee;color:#777}
.st-active{background:#e7f6ec;color:#0a7d33}
.hp-on_track{background:#e7f6ec;color:#0a7d33}.hp-at_risk{background:#fdf4e3;color:#b7791f}
.hp-blocked{background:#fdeaea;color:#c22}.hp-unknown{background:#f1f0ee;color:#777}
@media(prefers-color-scheme:dark){.pill{background:#232833!important;color:var(--fg)!important}}
small{color:var(--muted)}
footer{max-width:64rem;margin:1rem auto 3rem;padding:0 1.25rem;color:var(--muted);font-size:.8rem}
code{background:color-mix(in srgb,currentColor 12%,transparent);padding:.05rem .3rem;border-radius:4px}
td select{font-size:.78rem;padding:.15rem .3rem}
#toast{position:fixed;top:.7rem;left:50%;transform:translateX(-50%);z-index:20;display:flex;flex-direction:column;gap:.4rem;align-items:center}
.msg{padding:.4rem .8rem;border-radius:8px;font-size:.82rem;box-shadow:0 2px 10px rgba(0,0,0,.15)}
.msg.ok{background:#0a7d33;color:#fff}.msg.err{background:#c22;color:#fff}
`

const uiScript = `
function esc(s){return (s||'').replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]))}
function short(r){if(!r)return '';const p=r.split('/');return (p[p.length-1]||r).replace(/\.git$/,'')}
function sw(s){return (s||'').replace(/^(worker|agent|human):/,'')}
function pill(s){return '<span class="pill st-'+esc(s)+'">'+esc(s)+'</span>'}
const items=OBJS.filter(o=>o.type==='item'),projects=OBJS.filter(o=>o.type==='project');
function fill(id,vals){const el=document.getElementById(id);[...new Set(vals.filter(Boolean))].sort().forEach(v=>{const o=document.createElement('option');o.value=v;o.textContent=el.id+': '+v;el.appendChild(o)})}
fill('status',items.map(o=>o.status));fill('ty',items.map(o=>o.kind));
function stats(){const c=s=>items.filter(o=>o.status===s).length;
 document.getElementById('stats').innerHTML=[['projects',projects.length],['ready',c('ready')],['in progress',c('in_progress')],['blocked',c('blocked')],['review',c('review')],['attention',ATT.length]]
  .map(([k,v])=>'<span class="stat"><b>'+v+'</b> '+k+'</span>').join('')}
function renderAttn(){document.getElementById('attn-count').textContent=ATT.length?('· '+ATT.length):'';
 const el=document.getElementById('attn');
 if(!ATT.length){document.getElementById('attn-section').style.display='none';return}
 el.innerHTML=ATT.map(a=>'<div class="card sev-'+esc(a.severity)+'"><div class="t">'+esc(a.reason.replace(/_/g,' '))+'</div><div class="s">'+esc(a.summary)+'<br><code>'+esc(a.id)+'</code></div></div>').join('')}
const healthBy={};(HEALTH||[]).forEach(h=>healthBy[h.project]=h);
function hpill(id){const h=healthBy[id];if(!h)return '';const s=h.suggested_health;const why=(h.reasons||[]).join('; ');return '<span class="pill hp-'+esc(s)+'" title="'+esc(why)+'">'+esc(s.replace(/_/g,' '))+'</span>'}
function renderProjects(){document.getElementById('projects').innerHTML=projects.length?projects.map(p=>'<div class="card"><div class="t">'+esc(p.title)+' '+pill(p.status)+' '+hpill(p.id)+'</div><div class="s">'+esc(p.summary||'')+'<br>'+esc(short(p.target_repo))+'</div></div>').join(''):'<div class="s">No projects yet.</div>'}
const STATUSES=['inbox','triage','ready','in_progress','blocked','review','done','cancelled','archived'];
function setCtl(o){if(!WRITEABLE)return '';const opts=STATUSES.map(s=>'<option'+(s===o.status?' selected':'')+'>'+s+'</option>').join('');
 return '<td><select data-id="'+esc(o.id)+'" data-ver="'+esc(o.version||'')+'" onchange="wgSetStatus(this)">'+opts+'</select></td>'}
function renderItems(){const q=document.getElementById('q').value.toLowerCase(),st=document.getElementById('status').value,ty=document.getElementById('ty').value;
 const rows=items.filter(o=>(!st||o.status===st)&&(!ty||o.kind===ty)&&(!q||JSON.stringify(o).toLowerCase().includes(q)));
 const cols=WRITEABLE?6:5;
 document.querySelector('#objs tbody').innerHTML=rows.map(o=>'<tr><td>'+pill(o.status)+'</td><td>'+esc(o.title)+(o.summary?'<br><small>'+esc(o.summary)+'</small>':'')+'</td><td>'+esc(o.priority||'')+'</td><td>'+esc(sw(o.owner))+'</td><td>'+esc(short(o.target_repo))+'</td>'+setCtl(o)+'</tr>').join('')||'<tr><td colspan='+cols+'><small>No matching items.</small></td></tr>'}
function toast(msg,ok){const t=document.getElementById('toast');const d=document.createElement('div');d.className='msg '+(ok?'ok':'err');d.textContent=msg;t.appendChild(d);setTimeout(()=>d.remove(),3500)}
async function wgSetStatus(sel){const id=sel.dataset.id,status=sel.value,version=sel.dataset.ver;sel.disabled=true;
 try{const r=await fetch('wg/status',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id,status,version})});
  const j=await r.json();
  if(!r.ok){toast(j.error||('HTTP '+r.status),false);sel.disabled=false;return}
  toast(id+' → '+status,true);setTimeout(()=>location.reload(),400);
 }catch(e){toast(''+e,false);sel.disabled=false}}
function renderRuns(){const el=document.querySelector('#runs tbody');const rs=(RUNS||[]).slice().reverse();
 if(!rs.length){document.getElementById('runs-section').style.display='none';return}
 el.innerHTML=rs.map(r=>'<tr><td><code>'+esc(r.run)+'</code></td><td>'+(r.round||'')+'</td><td>'+esc(sw(r.worker))+'</td><td>'+pill(r.status||'')+'</td><td><small>'+esc(r.summary||'')+'</small></td></tr>').join('')}
function tldetail(r){let p=[];if(r.from||r.to)p.push((r.from||'∅')+'→'+(r.to||'∅'));if(r.status)p.push('['+r.status+']');if(r.message)p.push(r.message);return esc(p.join(' '))}
function renderTL(){const el=document.querySelector('#tl tbody');const rs=(TL||[]);
 if(!rs.length){document.getElementById('tl-section').style.display='none';return}
 el.innerHTML=rs.map(r=>'<tr><td><small>'+esc((r.at||'').slice(0,16).replace('T',' '))+'</small></td><td>'+esc(sw(r.actor))+'</td><td><small>'+esc(r.action)+'</small></td><td><small>'+esc(r.object||'')+'</small></td><td><small>'+tldetail(r)+'</small></td></tr>').join('')}
['q','status','ty'].forEach(id=>document.getElementById(id).addEventListener('input',renderItems));
document.querySelectorAll('#objs th[data-k]').forEach(th=>th.addEventListener('click',()=>{const k=th.dataset.k;items.sort((a,b)=>String(a[k]||'').localeCompare(String(b[k]||'')));renderItems()}));
stats();renderAttn();renderProjects();renderItems();renderRuns();renderTL();
if(LIVE&&window.EventSource){try{const es=new EventSource('wg/stream');let prev;
 es.onmessage=e=>{if(prev!==undefined&&e.data!==prev){location.reload()}prev=e.data};
 es.onerror=()=>{/* browser auto-reconnects */};}catch(e){}}
`

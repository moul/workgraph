package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moul/workgraph/internal/gateway"
)

func tokenDBPath(root string) string {
	return filepath.Join(root, ".workgraph", "gateway.db")
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", ":8080", "listen address")
	baseURL := fs.String("base-url", "", "public base URL (for token pages)")
	bootstrap := fs.Bool("bootstrap-admin-token", false, "mint and print a one-time bootstrap admin token")
	noPush := fs.Bool("no-push", false, "do not git push gateway mutations")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	ts, err := gateway.OpenTokenStore(tokenDBPath(ws.Root))
	if err != nil {
		return err
	}
	svc := &gateway.Service{Root: ws.Root, Tokens: ts, NoPush: *noPush}

	bootToken := ""
	if *bootstrap {
		buf := make([]byte, 24)
		_, _ = rand.Read(buf)
		bootToken = "wg_boot_" + hex.EncodeToString(buf)
		fmt.Println("bootstrap admin token (shown once):")
		fmt.Println("  " + bootToken)
		fmt.Println("mint scoped tokens with:")
		fmt.Printf("  curl -H 'Authorization: Bearer %s' -d '{\"kind\":\"run\",\"run\":\"RUN-...\",\"scopes\":[\"runs:context\",\"runs:event\",\"runs:finish\"]}' %s/admin/tokens\n", bootToken, orLocal(*baseURL, *addr))
	}

	srv := gateway.NewServer(svc, *addr, *baseURL, bootToken)
	fmt.Printf("workgraph gateway listening on %s (repo %s)\n", *addr, ws.Root)
	return srv.ListenAndServe()
}

func orLocal(base, addr string) string {
	if base != "" {
		return base
	}
	return "http://localhost" + addr
}

func cmdToken(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: workgraph token <create|list|revoke>")
	}
	switch args[0] {
	case "create":
		return tokenCreate(args[1:])
	case "list":
		return tokenList(args[1:])
	case "revoke":
		return tokenRevoke(args[1:])
	default:
		return fmt.Errorf("unknown token subcommand %q", args[0])
	}
}

func tokenCreate(args []string) error {
	fs := flag.NewFlagSet("token create", flag.ExitOnError)
	scope := fs.String("scope", "", "comma-separated scopes")
	item := fs.String("item", "", "item id")
	run := fs.String("run", "", "run id")
	project := fs.String("project", "", "project id")
	worker := fs.String("worker", "", "worker identity bound to the token")
	kind := fs.String("kind", "run", "token kind (run|item|workspace)")
	ttlHours := fs.Int("ttl-hours", 0, "override TTL in hours")
	admin := fs.Bool("admin", false, "grant admin capability")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	ts, err := gateway.OpenTokenStore(tokenDBPath(ws.Root))
	if err != nil {
		return err
	}
	var scopes []string
	if *scope != "" {
		scopes = strings.Split(*scope, ",")
	}
	ttl := gateway.DefaultTTL(*kind)
	if *ttlHours > 0 {
		ttl = time.Duration(*ttlHours) * time.Hour
	}
	raw, rec, err := ts.Mint(gateway.Token{Scopes: scopes, Item: *item, Run: *run, Project: *project, Worker: *worker, Admin: *admin, CreatedBy: defaultActor()}, ttl)
	if err != nil {
		return err
	}
	fmt.Printf("token created (shown once): %s\n", raw)
	fmt.Printf("  id:      %s\n", rec.ID)
	fmt.Printf("  scopes:  %s\n", strings.Join(rec.Scopes, ", "))
	if !rec.ExpiresAt.IsZero() {
		fmt.Printf("  expires: %s\n", rec.ExpiresAt.Format(time.RFC3339))
	}
	return nil
}

func tokenList(args []string) error {
	fs := flag.NewFlagSet("token list", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	ts, err := gateway.OpenTokenStore(tokenDBPath(ws.Root))
	if err != nil {
		return err
	}
	toks := ts.List()
	for i := range toks {
		toks[i].Hash = "" // never expose hashes
	}
	if *jsonOut {
		return printJSON(toks)
	}
	tw := newTab()
	fmt.Fprintln(tw, "ID\tSCOPES\tRUN\tREVOKED\tEXPIRES")
	for _, t := range toks {
		exp := "-"
		if !t.ExpiresAt.IsZero() {
			exp = t.ExpiresAt.Format("2006-01-02")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%v\t%s\n", t.ID, strings.Join(t.Scopes, ","), orDash(t.Run), t.Revoked, exp)
	}
	return tw.Flush()
}

func tokenRevoke(args []string) error {
	fs := flag.NewFlagSet("token revoke", flag.ExitOnError)
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)
	id, err := requireArg(fs, 0, "token id")
	if err != nil {
		return err
	}
	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	ts, err := gateway.OpenTokenStore(tokenDBPath(ws.Root))
	if err != nil {
		return err
	}
	if err := ts.Revoke(id); err != nil {
		return err
	}
	fmt.Printf("revoked %s\n", id)
	return nil
}

// cmdMCP runs the stdio MCP server or prints install instructions.
func cmdMCP(args []string) error {
	if len(args) > 0 && args[0] == "install" {
		return mcpInstall(args[1:])
	}
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	fs.StringVar(&o.actor, "actor", defaultActor(), "actor identity for events")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	svc := &gateway.Service{Root: ws.Root}
	h := &gateway.MCPHandler{Svc: svc, Actor: o.actor}

	// Newline-delimited JSON-RPC over stdio.
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	out := json.NewEncoder(os.Stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		resp := h.Dispatch(o.actor, []byte(line))
		if err := out.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

func mcpInstall(args []string) error {
	tool := "claude"
	if len(args) > 0 {
		tool = args[0]
	}
	bin, _ := os.Executable()
	if bin == "" {
		bin = "workgraph"
	}
	switch tool {
	case "claude":
		fmt.Printf("claude mcp add workgraph -- %s mcp\n", bin)
	case "codex":
		fmt.Printf("codex mcp add workgraph -- %s mcp\n", bin)
	default:
		return fmt.Errorf("unknown tool %q (want claude|codex)", tool)
	}
	fmt.Println("(this registers the local stdio MCP server; for remote, see `workgraph serve` + /setup/mcp)")
	return nil
}

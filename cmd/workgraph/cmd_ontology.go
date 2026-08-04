package main

import (
	"flag"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/moul/workgraph/internal/validate"
)

func cmdOntology(args []string) error {
	if len(args) == 0 {
		args = []string{"show"}
	}
	switch args[0] {
	case "show":
		return ontologyShow(args[1:])
	case "audit":
		return ontologyAudit(args[1:])
	default:
		return fmt.Errorf("unknown ontology subcommand %q (want show|audit)", args[0])
	}
}

func ontologyShow(args []string) error {
	fs := flag.NewFlagSet("ontology show", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(ws.Ontology)
	}
	raw, err := yaml.Marshal(ws.Ontology)
	if err != nil {
		return err
	}
	fmt.Print(string(raw))
	return nil
}

// ontologyAudit reports only the ontology-related findings from validation.
func ontologyAudit(args []string) error {
	fs := flag.NewFlagSet("ontology audit", flag.ExitOnError)
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	rep, err := validate.Run(ws)
	if err != nil {
		return err
	}
	n := 0
	for _, f := range rep.Findings {
		if containsAny(f.Message, "unknown object type", "unknown item kind", "unknown item status", "unknown decision status", "unknown relation", "unknown event action", "unknown capability", "unknown worker kind", "unknown health") {
			fmt.Println(f.String())
			n++
		}
	}
	fmt.Printf("\nontology audit: %d issue(s)\n", n)
	return nil
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

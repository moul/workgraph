package main

import (
	"flag"
	"fmt"

	"github.com/moul/workgraph/internal/gitutil"
	"github.com/moul/workgraph/internal/store"
)

func cmdMigrate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: workgraph migrate <plan|apply>")
	}
	switch args[0] {
	case "plan":
		return migratePlan(args[1:])
	case "apply":
		return migrateApply(args[1:])
	default:
		return fmt.Errorf("unknown migrate subcommand %q (want plan|apply)", args[0])
	}
}

func migratePlan(args []string) error {
	fs := flag.NewFlagSet("migrate plan", flag.ExitOnError)
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	steps, err := store.Plan(ws.Config.SchemaVersion)
	if err != nil {
		return err
	}
	if len(steps) == 0 {
		fmt.Printf("workspace is at schema %s — nothing to migrate.\n", store.CurrentSchema)
		return nil
	}
	fmt.Printf("plan: %s -> %s (%d step(s))\n", ws.Config.SchemaVersion, store.CurrentSchema, len(steps))
	for _, s := range steps {
		fmt.Printf("  %s -> %s\n", s.From, s.To)
	}
	fmt.Println("run `workgraph migrate apply` to write the changes in one commit.")
	return nil
}

func migrateApply(args []string) error {
	fs := flag.NewFlagSet("migrate apply", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	from := ws.Config.SchemaVersion
	steps, err := store.Plan(from)
	if err != nil {
		return err
	}
	if len(steps) == 0 {
		fmt.Printf("workspace is at schema %s — nothing to apply.\n", store.CurrentSchema)
		return nil
	}
	var changes []string
	for _, s := range steps {
		ch, err := s.Apply(ws)
		if err != nil {
			return fmt.Errorf("migration %s->%s failed: %w", s.From, s.To, err)
		}
		changes = append(changes, ch...)
	}
	ws.Config.SchemaVersion = store.CurrentSchema
	if err := ws.Config.Save(ws.Root); err != nil {
		return err
	}
	if repo := gitutil.Open(ws.Root); repo.IsRepo() && !o.noCommit {
		_, _ = repo.Commit(fmt.Sprintf("chore: migrate schema %s -> %s", from, store.CurrentSchema), store.ConfigFile)
	}
	fmt.Printf("migrated %s -> %s (%d change(s)).\n", from, store.CurrentSchema, len(changes))
	for _, c := range changes {
		fmt.Println("  " + c)
	}
	return nil
}

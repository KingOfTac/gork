package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"

	"github.com/apparentlymart/go-userdirs/userdirs"

	"github.com/kingoftac/flagon/cli"
	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/engine"
	"github.com/kingoftac/gork/internal/version"
)

var (
	dbName = "gork.db"
	dirs   = userdirs.ForApp("gork", "com.github.kingoftac.gork", "com.github.kingoftac.gork")
	dbPath = dirs.DataHome() + string(os.PathSeparator) + dbName
)

func main() {
	c := cli.New(&cli.Command{
		Name:        "gork",
		Description: "Workflow Orchestration Engine",
		Flags: func(fs *flag.FlagSet) {
			fs.Bool("version", false, "Print version information and exit")
		},
		Handler: func(ctx context.Context) error {
			showVersion := flag.Lookup("version")
			if showVersion != nil && showVersion.Value.String() == "true" {
				fmt.Println(version.Version)
				return nil
			}

			version.PrintBanner()
			return nil
		},
		Commands: []*cli.Command{
			{
				Name: "init-db",
				Handler: func(ctx context.Context) error {
					db, err := db.NewDB(dbPath)
					if err != nil {
						log.Fatal(err)
					}
					defer db.Close()

					fmt.Printf("Initialized database at %s\n", dbPath)
					return nil
				},
			},
			{
				Name: "create",
				Args: []cli.Arg{
					{
						Name:        "file",
						Description: "workflow yaml",
					},
				},
				Handler: func(ctx context.Context) error {
					file := cli.Args(ctx)[0]
					db, err := db.NewDB(dbPath)
					if err != nil {
						log.Fatal(err)
					}
					defer db.Close()

					eng := engine.NewEngine(db)
					workflow, err := eng.LoadWorkflow(file)
					if err != nil {
						log.Fatal(err)
					}

					if err := db.InsertWorkflow(workflow); err != nil {
						log.Fatal(err)
					}

					fmt.Printf("Created workflow %s\n", workflow.Name)

					return nil
				},
			},
			{
				Name: "list",
				Handler: func(ctx context.Context) error {
					db, err := db.NewDB(dbPath)
					if err != nil {
						log.Fatal(err)
					}
					defer db.Close()
					workflows, err := db.ListWorkflows()
					if err != nil {
						log.Fatal(err)
					}

					sort.Slice(workflows, func(i, j int) bool {
						return workflows[i].ID < workflows[j].ID
					})

					for _, w := range workflows {
						fmt.Printf("- %d: %s\n", w.ID, w.Name)
					}
					return nil
				},
			},
			{
				Name: "runs",
				Args: []cli.Arg{
					{Name: "workflow-id", Description: "ID of the workflow"},
				},
				Handler: func(ctx context.Context) error {
					id, err := strconv.ParseInt(cli.Args(ctx)[0], 10, 64)
					if err != nil {
						log.Fatal(err)
					}

					db, err := db.NewDB(dbPath)
					if err != nil {
						log.Fatal(err)
					}
					defer db.Close()
					runs, err := db.ListRuns(&id)
					if err != nil {
						log.Fatal(err)
					}
					if len(runs) == 0 {
						fmt.Println("No runs found")
						return nil
					}

					fmt.Printf("%-5s %-20s %-12s %-20s %-20s %-10s\n", "ID", "Workflow", "Status", "Started", "Completed", "Trigger")
					fmt.Println("--------------------------------------------------------------------------------")
					for _, r := range runs {
						workflowName := "Unknown"
						if &id == nil {
							workflow, err := db.GetWorkflow(r.WorkflowID)
							if err == nil {
								workflowName = workflow.Name
							}
						} else {
							workflow, err := db.GetWorkflow(id)
							if err == nil {
								workflowName = workflow.Name
							}
						}

						started := "N/A"
						if !r.StartedAt.IsZero() {
							started = r.StartedAt.Format("2006-01-02 15:04:05")
						}

						completed := "N/A"
						if !r.CompletedAt.IsZero() {
							completed = r.CompletedAt.Format("2006-01-02 15:04:05")
						}

						fmt.Printf("%-5d %-20s %-12s %-20s %-20s %-10s\n",
							r.ID,
							truncateString(workflowName, 20),
							r.Status,
							started,
							completed,
							r.Trigger)
					}

					return nil
				},
			},
		},
	}, cli.WithLogger(log.New(os.Stdout, "[gork] ", log.LstdFlags)))

	// c.Hook(cli.BeforeRun, func(ctx context.Context) error {
	// 	c.App().Logger.Println("running...")
	// 	return nil
	// })

	if err := c.Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

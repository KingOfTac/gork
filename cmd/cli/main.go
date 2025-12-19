package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/apparentlymart/go-userdirs/userdirs"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	"github.com/kingoftac/flagon/cli"
	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/engine"
	"github.com/kingoftac/gork/internal/fmtc"
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

					// Get terminal width, default to 120 if unavailable
					termWidth := 120
					if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
						termWidth = w
					}

					// Fixed column widths
					const (
						idWidth        = 6  // "ID" + padding
						statusWidth    = 12 // status text
						startedWidth   = 20 // datetime format
						completedWidth = 20 // datetime format
						triggerWidth   = 10 // trigger text
						spacing        = 5  // spaces between columns
					)

					// Calculate workflow name width (use remaining space)
					fixedWidth := idWidth + statusWidth + startedWidth + completedWidth + triggerWidth + spacing
					workflowWidth := termWidth - fixedWidth
					if workflowWidth < 15 {
						workflowWidth = 15 // minimum width for workflow name
					}
					if workflowWidth > 40 {
						workflowWidth = 40 // cap at reasonable max
					}

					// Build format strings
					headerFmt := fmt.Sprintf("{bg:white}{black}%%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds{reset}\n",
						idWidth-1, workflowWidth, statusWidth, startedWidth, completedWidth, triggerWidth)
					rowFmt := fmt.Sprintf("%%-%dd %%-%ds %%s %%-%ds %%-%ds %%-%ds\n",
						idWidth-1, workflowWidth, startedWidth, completedWidth, triggerWidth)

					// Print header
					fmtc.Printf(headerFmt, "ID", "Workflow", "Status", "Started", "Completed", "Trigger")
					fmt.Println(strings.Repeat("-", termWidth))

					sort.Slice(runs, func(i, j int) bool {
						return runs[i].ID < runs[j].ID
					})

					for _, r := range runs {
						workflowName := "Unknown"
						workflow, err := db.GetWorkflow(r.WorkflowID)
						if err == nil {
							workflowName = workflow.Name
						}

						started := "N/A"
						if !r.StartedAt.IsZero() {
							started = r.StartedAt.Format("2006-01-02 15:04:05")
						}

						completed := "N/A"
						if !r.CompletedAt.IsZero() {
							completed = r.CompletedAt.Format("2006-01-02 15:04:05")
						}

						status := string(r.Status)
						statusFmt := fmt.Sprintf("%%-%ds", statusWidth)
						paddedStatus := fmt.Sprintf(statusFmt, status)
						switch strings.ToLower(status) {
						case "pending":
							paddedStatus = fmtc.Sprintf("{bright:yellow}"+statusFmt+"{reset}", status)
						case "running":
							paddedStatus = fmtc.Sprintf("{bright:cyan}"+statusFmt+"{reset}", status)
						case "success", "succeeded", "completed":
							paddedStatus = fmtc.Sprintf("{bright:green}"+statusFmt+"{reset}", status)
						case "failed", "error":
							paddedStatus = fmtc.Sprintf("{bright:red}"+statusFmt+"{reset}", status)

							highlightRowFmt := fmtc.Sprintf("{bg:bright:red}{black}%%-%dd %%-%ds %%s %%-%ds %%-%ds %%-%ds{reset}\n",
								idWidth-1, workflowWidth, startedWidth, completedWidth, triggerWidth)
							paddedStatus = fmt.Sprintf(statusFmt, status)

							fmtc.Printf(highlightRowFmt,
								r.ID,
								truncateString(workflowName, workflowWidth),
								paddedStatus,
								started,
								completed,
								r.Trigger)
							continue
						default:
							paddedStatus = fmtc.Sprintf("{bright:white}"+statusFmt+"{reset}", status)
						}

						fmtc.Printf(rowFmt,
							r.ID,
							truncateString(workflowName, workflowWidth),
							paddedStatus,
							started,
							completed,
							r.Trigger)
					}

					return nil
				},
			},
			{
				Name: "run",
				Args: []cli.Arg{
					{Name: "workflow-name", Description: "Name of the workflow to run"},
				},
				Handler: func(ctx context.Context) error {
					name := cli.Args(ctx)[0]
					db, err := db.NewDB(dbPath)
					if err != nil {
						log.Fatal(err)
					}
					defer db.Close()

					workflow, err := db.GetWorkflowByName(name)
					if err != nil {
						log.Fatalf("Workflow %s not found", name)
					}

					eng := engine.NewEngine(db)
					run, err := eng.ExecuteWorkflow(context.Background(), workflow, "cli")
					if err != nil {
						log.Fatal(err)
					}
					fmt.Printf("Run %d completed with status %s\n", run.ID, run.Status)

					return nil
				},
			},
			{
				Name: "logs",
				Args: []cli.Arg{
					{Name: "run-id", Description: "ID of the workflow run"},
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

					stepRuns, err := db.GetStepRuns(id)
					if err != nil {
						log.Fatal(err)
					}

					for _, sr := range stepRuns {
						stepLine := fmt.Sprintf("Step: %s\n", sr.StepName)
						fmt.Printf(stepLine)
						fmt.Println(strings.Repeat("-", len(stepLine)-1))
						for i, log := range sr.Logs {
							fmt.Printf("[%d] %s\n", i+1, log)
						}
						fmt.Println()
					}

					return nil
				},
			},
			// {
			// 	Name: "watch",
			// 	Args: []cli.Arg{
			// 		{Name: "run-id", Description: "ID of the workflow run to watch"},
			// 	},
			// 	Handler: func(ctx context.Context) error {
			// 		id, err := strconv.ParseInt(cli.Args(ctx)[0], 10, 64)
			// 		if err != nil {
			// 			log.Fatal(err)
			// 		}

			// 		return nil
			// 	},
			// },
			{
				Name: "export",
				Args: []cli.Arg{
					{Name: "workflow-id", Description: "ID of the workflow to export"},
					{Name: "output-file", Description: "File to write the exported workflow YAML"},
				},
				Handler: func(ctx context.Context) error {
					id, err := strconv.ParseInt(cli.Args(ctx)[0], 10, 64)
					if err != nil {
						log.Fatal(err)
					}
					outputFile := cli.Args(ctx)[1]

					db, err := db.NewDB(dbPath)
					if err != nil {
						log.Fatal(err)
					}
					defer db.Close()

					workflow, err := db.GetWorkflow(id)
					if err != nil {
						log.Fatalf("Workflow with ID %d not found", id)
					}

					data, err := yaml.Marshal(workflow)
					if err != nil {
						log.Fatal(err)
					}

					if err := os.WriteFile(outputFile, data, 0644); err != nil {
						log.Fatal(err)
					}

					return nil
				},
			},
			{
				Name: "delete",
				Args: []cli.Arg{
					{Name: "workflow-id", Description: "ID of the workflow to delete"},
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

					if err := db.DeleteWorkflow(id); err != nil {
						log.Fatal(err)
					}

					fmt.Printf("Deleted workflow with ID %d\n", id)

					return nil
				},
			},
			{
				Name: "reset",
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

					totalRuns := 0
					for _, w := range workflows {
						runs, err := db.ListRuns(&w.ID)
						if err != nil {
							log.Fatal(err)
						}
						totalRuns += len(runs)
					}

					fmt.Printf("Are you sure you want to reset ALL data?\n")
					fmt.Printf("This will delete %d workflows and %d runs with all their step data.\n", len(workflows), totalRuns)
					fmt.Printf("This action CANNOT be undone!\n")
					fmtc.Printf("Type {bright:red}'RESET'{reset} to confirm: ")

					var response string
					fmt.Scanln(&response)
					if response != "RESET" {
						fmt.Println("Reset cancelled.")
						return nil
					}

					if err := db.ResetAllData(); err != nil {
						log.Fatal(err)
					}

					fmt.Printf("Successfully reset all data. Deleted %d workflows and %d runs.\n", len(workflows), totalRuns)

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

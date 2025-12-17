package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/apparentlymart/go-userdirs/userdirs"

	"github.com/kingoftac/flagon/cli"
	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/engine"
	"github.com/kingoftac/gork/internal/version"
)

var (
	dbName = "gork.db"
	dirs   = userdirs.ForApp("gork", "com.github.kingoftac.gork", "com.github.kingoftac.gork")
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
	}, cli.WithLogger(log.New(os.Stdout, "[gork] ", log.LstdFlags)))

	// c.Hook(cli.BeforeRun, func(ctx context.Context) error {
	// 	c.App().Logger.Println("running...")
	// 	return nil
	// })

	if err := c.Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	// global := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	// verbose := global.Bool("v", false, "Enable verbose output")
	// showVersion := global.Bool("version", false, "Print version information and exit")

	// global.Usage = func() {
	// 	printUsage()
	// }

	// if err := global.Parse(os.Args[1:]); err != nil {
	// 	os.Exit(1)
	// }

	// if *showVersion {
	// 	fmt.Println(version.Version)
	// 	os.Exit(0)
	// }

	// args := global.Args()
	// if len(args) == 0 {
	// 	version.PrintBanner()
	// 	printUsage()
	// 	os.Exit(0)
	// }

	// switch args[0] {
	// case "db":
	// 	handleDB(args[1:], *verbose)
	// case "workflow":
	// 	handleWorkflow(args[1:], *verbose)
	// default:
	// 	printUsage()
	// 	os.Exit(1)
	// }
}

func handleDB(args []string, verbose bool) {
	dBCmd := flag.NewFlagSet("db", flag.ExitOnError)

	dBCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, `
Usage:
  gork db <subcommand> [flags]

Subcommands:
  init                 Initialize the database
		`)
	}

	if len(args) == 0 {
		dBCmd.Usage()
		os.Exit(1)
	}

	switch args[0] {
	case "init":
		handleDBInit(args[1:], verbose)
	default:
		dBCmd.Usage()
		os.Exit(1)
	}
}

func handleDBInit(args []string, verbose bool) {
	dbPath := dirs.DataHome() + string(os.PathSeparator) + dbName
	log.Printf("Initializing database at %s\n", dbPath)
	db, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	fmt.Println("Database Initialized")
}

func handleWorkflow(args []string, verbose bool) {
	workflowCmd := flag.NewFlagSet("workflow", flag.ExitOnError)
	workflowCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, `
Usage:
  gork workflow <subcommand> [flags]

Subcommands:
  create               Creates a new workflow
  list                 Lists all workflows
	runs								 Lists all workflow runs
  logs                 Shows logs for a workflow run
  watch                Watches a workflow run in real-time
  export               Exports a workflow to a file
  delete               Deletes a workflow
		`)
	}

	if len(args) == 0 {
		workflowCmd.Usage()
		os.Exit(1)
	}

	switch args[0] {
	case "create":
		handleWorkflowCreate(args[1:], verbose)
	case "list":
		handleWorkflowList(args[1:], verbose)
	case "runs":
		handleWorkflowRuns(args[1:], verbose)
	case "run":
		handleWorkflowRun(args[1:], verbose)
	default:
		workflowCmd.Usage()
		os.Exit(1)
	}
}

func handleWorkflowCreate(args []string, verbose bool) {
	workflowCreateCmd := flag.NewFlagSet("create", flag.ExitOnError)
	workflowCreateCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, `
Usage:
  gork workflow create <file_path>
	gork workflow create <yaml>

Creates a new workflow from a file or raw YAML input
		`)
	}

	if len(args) == 0 {
		workflowCreateCmd.Usage()
		os.Exit(1)
	}

	createWorkflow(args[0])
}

func createWorkflow(file string) {
	dbPath := dirs.DataHome() + string(os.PathSeparator) + dbName

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
	fmt.Printf("Workflow '%s' created with ID %d\n", workflow.Name, workflow.ID)
}

func handleWorkflowList(args []string, verbose bool) {
	workflowListCmd := flag.NewFlagSet("list", flag.ExitOnError)
	workflowListCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, `
Usage:
	gork workflow list

Lists all workflows
		`)
	}

	if len(args) != 0 {
		workflowListCmd.Usage()
		os.Exit(1)
	}

	dbPath := dirs.DataHome() + string(os.PathSeparator) + dbName
	listWorkflows(dbPath)
}

func listWorkflows(dbPath string) {
	db, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	workflows, err := db.ListWorkflows()
	if err != nil {
		log.Fatal(err)
	}
	for _, w := range workflows {
		fmt.Printf("%d %s\n", w.ID, w.Name)
	}
}

func handleWorkflowRuns(args []string, verbose bool) {
	workflowRunsCmd := flag.NewFlagSet("runs", flag.ExitOnError)
	workflowRunsCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, `
Usage:
	gork workflow runs <id>

Lists all workflow runs for a given workflow ID
		`)
	}

	var workflowId *int64
	if len(args) == 1 {
		var id int64
		_, err := fmt.Sscanf(args[0], "%d", &id)
		if err != nil {
			log.Fatalf("Invalid workflow ID: %v", err)
		}
		workflowId = &id
	} else if len(args) > 1 {
		workflowRunsCmd.Usage()
		os.Exit(1)
	}

	workflowRunsCmd.Parse(args)

	dbPath := dirs.DataHome() + string(os.PathSeparator) + dbName

	listWorkflowRuns(dbPath, workflowId)
}

func listWorkflowRuns(dbPath string, workflowID *int64) {
	db, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	runs, err := db.ListRuns(workflowID)
	if err != nil {
		log.Fatal(err)
	}
	if len(runs) == 0 {
		fmt.Println("No runs found")
		return
	}
	fmt.Printf("%-5s %-20s %-12s %-20s %-20s %-10s\n", "ID", "Workflow", "Status", "Started", "Completed", "Trigger")
	fmt.Println("-----------------------------------------------------------------------------------------")
	for _, r := range runs {
		workflowName := "Unknown"
		if workflowID == nil {
			// Get workflow name
			workflow, err := db.GetWorkflow(r.WorkflowID)
			if err == nil {
				workflowName = workflow.Name
			}
		} else {
			workflow, err := db.GetWorkflow(*workflowID)
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
}

func handleWorkflowRun(args []string, verbose bool) {
	workflowRunCmd := flag.NewFlagSet("run", flag.ExitOnError)

	workflowRunCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, `
Usage:
	gork workflow run <workflow_name>

Runs a workflow by name
		`)
	}

	if len(args) == 0 {
		workflowRunCmd.Usage()
		os.Exit(1)
	}

	dbPath := dirs.DataHome() + string(os.PathSeparator) + dbName

	runWorkflow(dbPath, args[0])
}

func runWorkflow(dbPath, name string) {
	db, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	workflow, err := db.GetWorkflowByName(name)
	if err != nil {
		log.Fatalf("Workflow '%s' not found: %v", name, err)
	}
	eng := engine.NewEngine(db)
	run, err := eng.ExecuteWorkflow(context.Background(), workflow, "cli")
	if err != nil {
		log.Fatalf("Failed to run workflow '%s': %v", name, err)
	}
	fmt.Printf("Run %d completed with status %s\n", run.ID, run.Status)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

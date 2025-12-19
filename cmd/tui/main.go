package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/apparentlymart/go-userdirs/userdirs"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/tui"
)

var (
	dbName = "gork.db"
	dirs   = userdirs.ForApp("gork", "com.github.kingoftac.gork", "com.github.kingoftac.gork")
	dbPath = dirs.DataHome() + string(os.PathSeparator) + dbName
)

func findDaemonExecutable() string {
	// Determine executable name based on OS
	daemonName := "gork-daemon"
	if runtime.GOOS == "windows" {
		daemonName = "gork-daemon.exe"
	}

	// Check environment variable first
	if envPath := os.Getenv("GORK_DAEMON_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	// Get current working directory
	cwd, _ := os.Getwd()

	// Get executable directory
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	// List of paths to check
	searchPaths := []string{
		// Current working directory
		filepath.Join(cwd, daemonName),
		// Same directory as TUI executable
		filepath.Join(exeDir, daemonName),
		// cmd/daemon directory (development)
		filepath.Join(cwd, "cmd", "daemon", daemonName),
		// Build output directory
		filepath.Join(cwd, "bin", daemonName),
		// Go bin directory
		filepath.Join(os.Getenv("GOPATH"), "bin", daemonName),
		filepath.Join(os.Getenv("GOBIN"), daemonName),
	}

	// Check each path
	for _, path := range searchPaths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Return default name and let it fail later with a clear error
	return daemonName
}

func main() {
	database, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	daemonPath := findDaemonExecutable()

	model := tui.NewModel(database, daemonPath)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

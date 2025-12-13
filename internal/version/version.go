package version

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func PrintBanner() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                              ║")
	fmt.Println("║               ██████╗  ██████╗ ██████╗ ██╗  ██╗              ║")
	fmt.Println("║              ██╔════╝ ██╔═══██╗██╔══██╗██║ ██╔╝              ║")
	fmt.Println("║              ██║  ███╗██║   ██║██████╔╝█████╔╝               ║")
	fmt.Println("║              ██║   ██║██║   ██║██╔══██╗██╔═██╗               ║")
	fmt.Println("║              ╚██████╔╝╚██████╔╝██║  ██║██║  ██╗              ║")
	fmt.Println("║               ╚═════╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝              ║")
	fmt.Println("║                                                              ║")
	fmt.Println("║                 Workflow Orchestration Engine                ║")
	fmt.Println("║                                                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Printf("Version: %s | https://github.com/KingOfTac/go-workflow\n\n", Version)
}

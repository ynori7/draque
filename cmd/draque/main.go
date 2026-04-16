package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	state := &appState{}

	fmt.Println("Draque — API endpoint discovery tool")
	fmt.Println("Type 'help' for available commands.")

	for {
		fmt.Print("\ndraque> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF (Ctrl+D) — exit cleanly
			fmt.Println()
			return
		}

		cmd := strings.TrimSpace(line)
		if cmd == "" {
			continue
		}

		switch cmd {
		case "help", "h", "?":
			printHelp()
		case "wayback", "w":
			cmdWayback(reader, state)
		case "logs", "log", "l":
			cmdLogs(reader, state)
		case "swagger", "sw":
			cmdSwagger(reader, state)
		case "status":
			cmdStatus(state)
		case "scan", "s":
			cmdScan(state)
		case "analyze", "a":
			cmdAnalyze(state)
		case "search":
			cmdSearch(reader, state)
		case "export", "e":
			cmdExport(reader, state)
		case "quit", "exit", "q":
			fmt.Println("Goodbye.")
			os.Exit(0)
		default:
			fmt.Printf("  Unknown command %q. Type 'help' for available commands.\n", cmd)
		}
	}
}

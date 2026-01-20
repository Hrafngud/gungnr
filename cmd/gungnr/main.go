package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gungnr-cli/internal/cli/app"
	"gungnr-cli/internal/cli/tui"
	cliui "gungnr-cli/internal/cli/ui"
)

//go:embed ascii-art.txt
var logoArt string

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "bootstrap":
		runBootstrap(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  gungnr bootstrap [--plain]")
}

func runBootstrap(args []string) {
	flags := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	plain := flags.Bool("plain", false, "Disable the TUI and use plain console output")
	flags.BoolVar(plain, "no-tui", false, "Disable the TUI and use plain console output")
	_ = flags.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	steps := app.BootstrapSteps()

	if *plain {
		consoleUI := cliui.NewConsoleUI(os.Stdout, os.Stderr, os.Stdin)
		runner := app.NewRunner(consoleUI, steps)
		if err := runner.Run(ctx); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	tuiUI := tui.New(logoArt)
	runner := app.NewRunner(tuiUI, steps)
	if err := tuiUI.Run(ctx, runner.Run); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

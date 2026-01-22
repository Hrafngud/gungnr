package main

import (
	"bufio"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
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
	case "restart":
		runRestart(os.Args[2:])
	case "tunnel":
		runTunnel(os.Args[2:])
	case "uninstall":
		runUninstall(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  gungnr bootstrap [--plain]")
	fmt.Fprintln(os.Stderr, "  gungnr restart")
	fmt.Fprintln(os.Stderr, "  gungnr tunnel run")
	fmt.Fprintln(os.Stderr, "  gungnr uninstall [--yes]")
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

func runRestart(args []string) {
	flags := flag.NewFlagSet("restart", flag.ExitOnError)
	_ = flags.Parse(args)

	if err := app.Restart(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runTunnel(args []string) {
	flags := flag.NewFlagSet("tunnel", flag.ExitOnError)
	_ = flags.Parse(args)

	subcommand := ""
	if flags.NArg() > 0 {
		subcommand = flags.Arg(0)
	}

	switch subcommand {
	case "run":
		logPath, err := app.RunTunnel()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "Cloudflared tunnel started. Logs: %s\n", logPath)
	default:
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  gungnr tunnel run")
		os.Exit(2)
	}
}

func runUninstall(args []string) {
	flags := flag.NewFlagSet("uninstall", flag.ExitOnError)
	yes := flags.Bool("yes", false, "Skip confirmation prompt")
	flags.BoolVar(yes, "y", false, "Skip confirmation prompt")
	_ = flags.Parse(args)

	plan, err := app.BuildUninstallPlan()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if len(plan.Targets) == 0 {
		fmt.Fprintln(os.Stdout, "Nothing to remove.")
		return
	}

	fmt.Fprintln(os.Stdout, "The following Gungnr files/directories will be removed:")
	for _, target := range plan.Targets {
		fmt.Fprintf(os.Stdout, "- %s\n", target)
	}

	if !*yes {
		fmt.Fprint(os.Stdout, "Proceed? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Fprintln(os.Stdout, "Aborted.")
			return
		}
	}

	if err := app.ExecuteUninstall(plan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, "Gungnr uninstall complete.")
}

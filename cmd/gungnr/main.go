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
	"time"

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
	case "version", "--version", "-v":
		printVersion()
	case "bootstrap":
		runBootstrap(os.Args[2:])
	case "restart":
		runRestart(os.Args[2:])
	case "tunnel":
		runTunnel(os.Args[2:])
	case "keepalive":
		runKeepalive(os.Args[2:])
	case "netbird":
		runNetBird(os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "  gungnr version")
	fmt.Fprintln(os.Stderr, "  gungnr bootstrap [--plain]")
	fmt.Fprintln(os.Stderr, "  gungnr restart")
	fmt.Fprintln(os.Stderr, "  gungnr tunnel run")
	fmt.Fprintln(os.Stderr, "  gungnr keepalive")
	fmt.Fprintln(os.Stderr, "  gungnr netbird mode --set <legacy|mode_a|mode_b> [--allow-localhost]")
	fmt.Fprintln(os.Stderr, "  gungnr netbird status")
	fmt.Fprintln(os.Stderr, "  gungnr uninstall [--yes]")
}

var (
	version = "dev"
	commit  = "local"
	date    = "unknown"
)

func printVersion() {
	info := buildVersionInfo()
	fmt.Fprintf(os.Stdout, "gungnr %s (commit %s, built %s)\n", info.Version, info.Commit, info.Date)
}

type buildInfo struct {
	Version string
	Commit  string
	Date    string
}

func buildVersionInfo() buildInfo {
	return buildInfo{
		Version: normalizeBuildValue(version, "dev"),
		Commit:  normalizeBuildValue(commit, "local"),
		Date:    normalizeBuildValue(date, "unknown"),
	}
}

func normalizeBuildValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
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

func runKeepalive(args []string) {
	if len(args) != 0 {
		printKeepaliveUsage()
		os.Exit(2)
	}

	var (
		output string
		err    error
	)

	trigger := strings.TrimSpace(os.Getenv("GUNGNR_KEEPALIVE_TRIGGER"))
	if trigger == "" {
		output, err = app.KeepaliveToggle()
	} else {
		output, err = app.KeepaliveRecover(trigger)
	}

	if strings.TrimSpace(output) != "" {
		fmt.Fprintln(os.Stdout, output)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printKeepaliveUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  gungnr keepalive")
}

func runNetBird(args []string) {
	if len(args) == 0 {
		printNetBirdUsage()
		os.Exit(2)
	}

	switch args[0] {
	case "mode":
		runNetBirdMode(args[1:])
	case "status":
		runNetBirdStatus(args[1:])
	default:
		printNetBirdUsage()
		os.Exit(2)
	}
}

func runNetBirdMode(args []string) {
	flags := flag.NewFlagSet("netbird mode", flag.ExitOnError)
	targetMode := flags.String("set", "", "Target mode: legacy|mode_a|mode_b")
	allowLocalhost := flags.Bool("allow-localhost", false, "Allow localhost listener in NetBird mode")
	panelAPIURL := flags.String("api-url", "", "Panel API base URL (default: http://localhost)")
	panelAuthToken := flags.String("auth-token", "", "Panel bearer auth token")
	netbirdAPIBaseURL := flags.String("netbird-api-base-url", "", "NetBird API base URL override passed to backend apply")
	netbirdAPIToken := flags.String("netbird-api-token", "", "NetBird API token used for backend apply")
	hostPeerID := flags.String("host-peer-id", "", "NetBird host peer ID (required for mode_a/mode_b)")
	adminPeerIDs := flags.String("admin-peer-ids", "", "Comma-separated NetBird admin peer IDs (required for mode_a/mode_b)")
	autoApprove := flags.Bool("yes", false, "Skip interactive apply confirmation prompt")
	pollInterval := flags.Duration("poll-interval", 2*time.Second, "Job polling interval")
	pollTimeout := flags.Duration("poll-timeout", 30*time.Minute, "Max wait time for mode apply job completion")
	_ = flags.Parse(args)

	if strings.TrimSpace(*targetMode) == "" {
		printNetBirdModeUsage()
		os.Exit(2)
	}
	if flags.NArg() > 0 {
		printNetBirdModeUsage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.NetBirdModeSwitch(ctx, app.NetBirdModeSwitchOptions{
		TargetMode:        *targetMode,
		AllowLocalhost:    *allowLocalhost,
		PanelAPIBaseURL:   *panelAPIURL,
		PanelAuthToken:    *panelAuthToken,
		NetBirdAPIBaseURL: *netbirdAPIBaseURL,
		NetBirdAPIToken:   *netbirdAPIToken,
		HostPeerID:        *hostPeerID,
		AdminPeerIDs:      parseCSVArg(*adminPeerIDs),
		AutoApprove:       *autoApprove,
		PollInterval:      *pollInterval,
		PollTimeout:       *pollTimeout,
		Stdin:             os.Stdin,
		Stdout:            os.Stdout,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printNetBirdUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  gungnr netbird mode --set <legacy|mode_a|mode_b> [--allow-localhost]")
	fmt.Fprintln(os.Stderr, "  gungnr netbird status [--api-url URL] [--auth-token TOKEN]")
}

func printNetBirdModeUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  gungnr netbird mode --set <legacy|mode_a|mode_b> [--allow-localhost] [--api-url URL] [--auth-token TOKEN] [--netbird-api-base-url URL] [--netbird-api-token TOKEN] [--host-peer-id ID] [--admin-peer-ids id1,id2] [--yes] [--poll-interval 2s] [--poll-timeout 30m]")
}

func runNetBirdStatus(args []string) {
	flags := flag.NewFlagSet("netbird status", flag.ExitOnError)
	panelAPIURL := flags.String("api-url", "", "Panel API base URL (default: http://localhost)")
	panelAuthToken := flags.String("auth-token", "", "Panel bearer auth token")
	_ = flags.Parse(args)

	if flags.NArg() > 0 {
		printNetBirdUsage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.NetBirdStatus(ctx, app.NetBirdStatusOptions{
		PanelAPIBaseURL: *panelAPIURL,
		PanelAuthToken:  *panelAuthToken,
		Stdout:          os.Stdout,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseCSVArg(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
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

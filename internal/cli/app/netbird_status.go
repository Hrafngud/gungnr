package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gungnr-cli/internal/cli/integrations/panelapi"
)

type NetBirdStatusOptions struct {
	PanelAPIBaseURL string
	PanelAuthToken  string
	Stdout          io.Writer
}

func NetBirdStatus(ctx context.Context, options NetBirdStatusOptions) error {
	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	panelBaseURL, err := resolvePanelAPIBaseURL(options.PanelAPIBaseURL)
	if err != nil {
		return err
	}
	client, err := panelapi.NewClient(panelBaseURL)
	if err != nil {
		return err
	}

	panelAuthToken, authSource, err := resolvePanelAuthToken(ctx, client, strings.TrimSpace(options.PanelAuthToken))
	if err != nil {
		return err
	}
	client.SetAuthToken(panelAuthToken)

	status, err := client.GetNetBirdStatus(ctx)
	if err != nil {
		return fmt.Errorf("netbird status request failed: %w", err)
	}

	printNetBirdStatusSummary(stdout, panelBaseURL, authSource, status)
	return nil
}

func printNetBirdStatusSummary(stdout io.Writer, panelBaseURL, authSource string, status panelapi.NetBirdStatus) {
	fmt.Fprintln(stdout, "NetBird Status")
	fmt.Fprintf(stdout, "  Panel API: %s\n", strings.TrimSpace(panelBaseURL))
	fmt.Fprintf(stdout, "  Panel auth: %s\n", strings.TrimSpace(authSource))

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Connectivity")
	fmt.Fprintf(stdout, "  clientInstalled: %t\n", status.ClientInstalled)
	fmt.Fprintf(stdout, "  daemonRunning: %t\n", status.DaemonRunning)
	fmt.Fprintf(stdout, "  connected: %t\n", status.Connected)
	fmt.Fprintf(stdout, "  peerId: %s\n", valueOrPlaceholder(status.PeerID, "(not reported)"))
	fmt.Fprintf(stdout, "  peerName: %s\n", valueOrPlaceholder(status.PeerName, "(not reported)"))
	fmt.Fprintf(stdout, "  wg0Ip: %s\n", valueOrPlaceholder(status.WG0IP, "(not reported)"))

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Policy Sync")
	fmt.Fprintf(stdout, "  currentMode: %s\n", valueOrPlaceholder(status.CurrentMode, "unknown"))
	fmt.Fprintf(stdout, "  lastPolicySyncAt: %s\n", formatTimestampPointer(status.LastPolicySyncAt))
	fmt.Fprintf(stdout, "  lastPolicySyncStatus: %s\n", valueOrPlaceholder(status.LastPolicySyncStatus, "unknown"))
	fmt.Fprintf(stdout, "  warningCount: %d\n", status.LastPolicySyncWarning)

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Managed Resources")
	fmt.Fprintf(stdout, "  managedGroups: %d\n", status.ManagedGroups)
	fmt.Fprintf(stdout, "  managedPolicies: %d\n", status.ManagedPolicies)
	fmt.Fprintf(stdout, "  apiReachable: %t\n", status.APIReachable)
	fmt.Fprintf(stdout, "  apiReachabilitySource: %s\n", valueOrPlaceholder(status.APIReachability.Source, "unknown"))
	fmt.Fprintf(stdout, "  apiReachabilityMessage: %s\n", valueOrPlaceholder(status.APIReachability.Message, "(none)"))
}

func valueOrPlaceholder(raw string, fallback string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func formatTimestampPointer(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "never"
	}
	return value.UTC().Format(time.RFC3339)
}

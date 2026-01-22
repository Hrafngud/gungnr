package service

import (
	"context"
	"fmt"
	"strings"

	"go-notes/internal/integrations/cloudflare"
)

type CloudflareCheck struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type CloudflarePreflight struct {
	Token         CloudflareCheck `json:"token"`
	Account       CloudflareCheck `json:"account"`
	Zone          CloudflareCheck `json:"zone"`
	Tunnel        CloudflareCheck `json:"tunnel"`
	TunnelRef     string          `json:"tunnelRef,omitempty"`
	TunnelRefType string          `json:"tunnelRefType,omitempty"`
}

type CloudflareService struct {
	settings *SettingsService
}

func NewCloudflareService(settings *SettingsService) *CloudflareService {
	return &CloudflareService{settings: settings}
}

func (s *CloudflareService) Preflight(ctx context.Context) (CloudflarePreflight, error) {
	if s.settings == nil {
		return CloudflarePreflight{}, fmt.Errorf("settings service unavailable")
	}
	cfg, _, err := s.settings.ResolveConfigWithSources(ctx)
	if err != nil {
		return CloudflarePreflight{}, err
	}

	result := CloudflarePreflight{
		Token:   CloudflareCheck{Status: "missing", Detail: "Cloudflare API token is not set."},
		Account: CloudflareCheck{Status: "missing", Detail: "Cloudflare account ID is not set."},
		Zone:    CloudflareCheck{Status: "missing", Detail: "Cloudflare zone ID is not set."},
		Tunnel:  CloudflareCheck{Status: "missing", Detail: "Cloudflared tunnel name or ID is not set."},
	}

	token := strings.TrimSpace(cfg.CloudflareAPIToken)
	accountID := strings.TrimSpace(cfg.CloudflareAccountID)
	zoneID := strings.TrimSpace(cfg.CloudflareZoneID)

	client := cloudflare.NewClient(cfg)
	if token == "" {
		result.Token = CloudflareCheck{Status: "missing", Detail: "Cloudflare API token is not set."}
		result.Account = CloudflareCheck{Status: "skipped", Detail: "Set a token to validate account access."}
		result.Zone = CloudflareCheck{Status: "skipped", Detail: "Set a token to validate zone access."}
	} else {
		tokenStatus, err := client.VerifyToken(ctx)
		if err != nil {
			result.Token = CloudflareCheck{Status: "error", Detail: err.Error()}
		} else {
			detail := "Token verified"
			if tokenStatus.Status != "" {
				detail = fmt.Sprintf("Token %s", tokenStatus.Status)
			}
			result.Token = CloudflareCheck{Status: "ok", Detail: detail}
		}

		var zoneInfo cloudflare.ZoneInfo
		if zoneID == "" {
			result.Zone = CloudflareCheck{Status: "missing", Detail: "Cloudflare zone ID is not set."}
		} else {
			zoneInfo, err = client.Zone(ctx, zoneID)
			if err != nil {
				result.Zone = CloudflareCheck{Status: "error", Detail: err.Error()}
			} else {
				detail := "Zone access ok"
				if zoneInfo.Name != "" {
					detail = fmt.Sprintf("Zone %s", zoneInfo.Name)
				}
				result.Zone = CloudflareCheck{Status: "ok", Detail: detail}
			}
		}

		if accountID == "" {
			detail := "Cloudflare account ID is not set."
			if zoneInfo.Account.ID != "" {
				detail = fmt.Sprintf("Set account ID. Zone belongs to %s.", zoneInfo.Account.ID)
			}
			result.Account = CloudflareCheck{Status: "missing", Detail: detail}
		} else if zoneInfo.Account.ID != "" && !strings.EqualFold(zoneInfo.Account.ID, accountID) {
			result.Account = CloudflareCheck{
				Status: "error",
				Detail: fmt.Sprintf("Zone belongs to account %s, not %s.", zoneInfo.Account.ID, accountID),
			}
		} else if zoneInfo.Account.ID != "" {
			detail := "Account access ok"
			if zoneInfo.Account.Name != "" {
				detail = fmt.Sprintf("Account %s", zoneInfo.Account.Name)
			}
			result.Account = CloudflareCheck{Status: "ok", Detail: detail}
		} else if result.Zone.Status == "error" {
			result.Account = CloudflareCheck{Status: "skipped", Detail: "Fix the zone error to validate account access."}
		} else {
			result.Account = CloudflareCheck{Status: "ok", Detail: "Account access ok"}
		}
	}

	tunnelRef := strings.TrimSpace(cfg.CloudflaredTunnel)
	if tunnelRef == "" {
		if configID, err := client.TunnelIDFromConfig(); err == nil && configID != "" {
			tunnelRef = configID
		}
	}
	if tunnelRef == "" && looksLikeUUID(cfg.CloudflareTunnelID) {
		tunnelRef = strings.TrimSpace(cfg.CloudflareTunnelID)
	}
	result.TunnelRef = tunnelRef
	result.TunnelRefType = tunnelRefType(tunnelRef)
	switch result.TunnelRefType {
	case "":
		result.Tunnel = CloudflareCheck{Status: "missing", Detail: "Cloudflared tunnel name or ID is not set."}
	case "name":
		result.Tunnel = CloudflareCheck{
			Status: "warning",
			Detail: "Tunnel reference is a name; set the tunnel UUID to avoid /tunnels lookups.",
		}
	case "id":
		result.Tunnel = CloudflareCheck{Status: "ok", Detail: "Tunnel ID configured."}
	default:
		result.Tunnel = CloudflareCheck{Status: "warning", Detail: "Tunnel reference format is unexpected."}
	}

	return result, nil
}

func (s *CloudflareService) Zones(ctx context.Context) ([]cloudflare.ZoneInfo, error) {
	if s.settings == nil {
		return nil, fmt.Errorf("settings service unavailable")
	}
	cfg, _, err := s.settings.ResolveConfigWithSources(ctx)
	if err != nil {
		return nil, err
	}
	client := cloudflare.NewClient(cfg)
	return client.ListZones(ctx)
}

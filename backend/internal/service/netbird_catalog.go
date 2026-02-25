package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go-notes/internal/errs"
)

type NetBirdMode string

const (
	NetBirdModeLegacy NetBirdMode = "legacy"
	NetBirdModeA      NetBirdMode = "mode_a"
	NetBirdModeB      NetBirdMode = "mode_b"
)

const (
	netBirdGroupPanelName      = "gungnr-panel"
	netBirdGroupAdminsName     = "gungnr-admins"
	netBirdModeAPolicyName     = "gungnr-mode-a-admins-to-panel"
	netBirdModeBPanelPolicy    = "gungnr-mode-b-admins-to-panel"
	netBirdModeBProjectPrefix  = "gungnr-mode-b-admins-to-project-"
	netBirdProjectGroupPrefix  = "gungnr-project-"
	netBirdDefaultPolicyName   = "Default"
	netBirdActionAccept        = "accept"
	netBirdProtocolTCP         = "tcp"
	netBirdHostPeerPlaceholder = "${HOST_PEER_ID}"
)

const (
	netBirdAdminsPeerPlaceholderA = "${ADMIN_PEER_ID_1}"
	netBirdAdminsPeerPlaceholderB = "${ADMIN_PEER_ID_N}"
	netBirdGroupIDAdmins          = "${GROUP_ID_GUNGNR_ADMINS}"
	netBirdGroupIDPanel           = "${GROUP_ID_GUNGNR_PANEL}"
)

const (
	defaultPanelPort   = 8080
	defaultIngressPort = 80
)

type NetBirdGroupPayload struct {
	Name  string   `json:"name"`
	Peers []string `json:"peers"`
}

type NetBirdPolicyPayload struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	Enabled     bool                    `json:"enabled"`
	Rules       []NetBirdPolicyRuleSpec `json:"rules"`
}

type NetBirdPolicyRuleSpec struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Enabled       bool     `json:"enabled"`
	Action        string   `json:"action"`
	Bidirectional bool     `json:"bidirectional"`
	Protocol      string   `json:"protocol"`
	Ports         []string `json:"ports"`
	Sources       []string `json:"sources"`
	Destinations  []string `json:"destinations"`
}

type NetBirdCatalog struct {
	Groups   []NetBirdGroupPayload  `json:"groups"`
	Policies []NetBirdPolicyPayload `json:"policies"`
}

type NetBirdProjectCatalogInput struct {
	ProjectID   uint   `json:"projectId"`
	ProjectName string `json:"projectName"`
	IngressPort int    `json:"ingressPort"`
}

type NetBirdCatalogInput struct {
	Mode      NetBirdMode                  `json:"mode"`
	PanelPort int                          `json:"panelPort"`
	Projects  []NetBirdProjectCatalogInput `json:"projects"`
}

func ParseNetBirdMode(raw string) (NetBirdMode, error) {
	switch normalizeMode(raw) {
	case NetBirdModeLegacy:
		return NetBirdModeLegacy, nil
	case NetBirdModeA:
		return NetBirdModeA, nil
	case NetBirdModeB:
		return NetBirdModeB, nil
	default:
		return "", errs.New(errs.CodeNetBirdInvalidMode, fmt.Sprintf("unsupported target mode %q", strings.TrimSpace(raw)))
	}
}

func normalizeMode(raw string) NetBirdMode {
	return NetBirdMode(strings.ToLower(strings.TrimSpace(raw)))
}

func BuildNetBirdCatalog(input NetBirdCatalogInput) NetBirdCatalog {
	panelPort := input.PanelPort
	if panelPort <= 0 {
		panelPort = defaultPanelPort
	}
	projects := normalizeCatalogProjects(input.Projects)

	switch input.Mode {
	case NetBirdModeA:
		return NetBirdCatalog{
			Groups: []NetBirdGroupPayload{
				basePanelGroup(),
				baseAdminsGroup(),
			},
			Policies: []NetBirdPolicyPayload{
				modeAPanelPolicy(panelPort),
			},
		}
	case NetBirdModeB:
		groups := []NetBirdGroupPayload{
			basePanelGroup(),
			baseAdminsGroup(),
		}
		for _, project := range projects {
			groups = append(groups, projectGroup(project.ProjectID))
		}

		policies := []NetBirdPolicyPayload{
			modeBPanelPolicy(panelPort),
		}
		for _, project := range projects {
			policies = append(policies, modeBProjectPolicy(project.ProjectID, project.IngressPort))
		}
		return NetBirdCatalog{
			Groups:   groups,
			Policies: policies,
		}
	default:
		return NetBirdCatalog{
			Groups:   []NetBirdGroupPayload{},
			Policies: []NetBirdPolicyPayload{},
		}
	}
}

func normalizeCatalogProjects(projects []NetBirdProjectCatalogInput) []NetBirdProjectCatalogInput {
	out := make([]NetBirdProjectCatalogInput, 0, len(projects))
	for _, project := range projects {
		port := project.IngressPort
		if port <= 0 {
			port = defaultIngressPort
		}
		out = append(out, NetBirdProjectCatalogInput{
			ProjectID:   project.ProjectID,
			ProjectName: strings.TrimSpace(project.ProjectName),
			IngressPort: port,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ProjectID == out[j].ProjectID {
			return out[i].ProjectName < out[j].ProjectName
		}
		return out[i].ProjectID < out[j].ProjectID
	})
	return out
}

func basePanelGroup() NetBirdGroupPayload {
	return NetBirdGroupPayload{
		Name:  netBirdGroupPanelName,
		Peers: []string{netBirdHostPeerPlaceholder},
	}
}

func baseAdminsGroup() NetBirdGroupPayload {
	return NetBirdGroupPayload{
		Name:  netBirdGroupAdminsName,
		Peers: []string{netBirdAdminsPeerPlaceholderA, netBirdAdminsPeerPlaceholderB},
	}
}

func modeAPanelPolicy(panelPort int) NetBirdPolicyPayload {
	return NetBirdPolicyPayload{
		Name:        netBirdModeAPolicyName,
		Description: "Gungnr managed Mode A policy: admins can reach panel port only",
		Enabled:     true,
		Rules: []NetBirdPolicyRuleSpec{
			{
				Name:          "allow-admins-to-panel",
				Description:   "Unidirectional admin access to panel listener on wg0",
				Enabled:       true,
				Action:        netBirdActionAccept,
				Bidirectional: false,
				Protocol:      netBirdProtocolTCP,
				Ports:         []string{strconv.Itoa(panelPort)},
				Sources:       []string{netBirdGroupIDAdmins},
				Destinations:  []string{netBirdGroupIDPanel},
			},
		},
	}
}

func modeBPanelPolicy(panelPort int) NetBirdPolicyPayload {
	return NetBirdPolicyPayload{
		Name:        netBirdModeBPanelPolicy,
		Description: "Gungnr managed Mode B policy: admins can reach panel only on panel port",
		Enabled:     true,
		Rules: []NetBirdPolicyRuleSpec{
			{
				Name:          "allow-admins-to-panel",
				Enabled:       true,
				Action:        netBirdActionAccept,
				Bidirectional: false,
				Protocol:      netBirdProtocolTCP,
				Ports:         []string{strconv.Itoa(panelPort)},
				Sources:       []string{netBirdGroupIDAdmins},
				Destinations:  []string{netBirdGroupIDPanel},
			},
		},
	}
}

func modeBProjectPolicy(projectID uint, ingressPort int) NetBirdPolicyPayload {
	if ingressPort <= 0 {
		ingressPort = defaultIngressPort
	}

	return NetBirdPolicyPayload{
		Name:        netBirdModeBProjectPolicyName(projectID),
		Description: "Gungnr managed Mode B policy: admins can reach project ingress only",
		Enabled:     true,
		Rules: []NetBirdPolicyRuleSpec{
			{
				Name:          netBirdModeBProjectRuleName(projectID),
				Enabled:       true,
				Action:        netBirdActionAccept,
				Bidirectional: false,
				Protocol:      netBirdProtocolTCP,
				Ports:         []string{strconv.Itoa(ingressPort)},
				Sources:       []string{netBirdGroupIDAdmins},
				Destinations:  []string{netBirdProjectGroupID(projectID)},
			},
		},
	}
}

func projectGroup(projectID uint) NetBirdGroupPayload {
	return NetBirdGroupPayload{
		Name:  netBirdProjectGroupName(projectID),
		Peers: []string{netBirdHostPeerPlaceholder},
	}
}

func netBirdProjectGroupName(projectID uint) string {
	return fmt.Sprintf("%s%d", netBirdProjectGroupPrefix, projectID)
}

func netBirdModeBProjectPolicyName(projectID uint) string {
	return fmt.Sprintf("%s%d", netBirdModeBProjectPrefix, projectID)
}

func netBirdModeBProjectRuleName(projectID uint) string {
	return fmt.Sprintf("allow-admins-to-project-%d", projectID)
}

func netBirdProjectGroupID(projectID uint) string {
	return fmt.Sprintf("${GROUP_ID_GUNGNR_PROJECT_%d}", projectID)
}

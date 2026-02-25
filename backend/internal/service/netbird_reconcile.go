package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go-notes/internal/errs"
	netbirdapi "go-notes/internal/integrations/netbird"
)

const (
	netBirdResultCreated   = "created"
	netBirdResultUpdated   = "updated"
	netBirdResultDeleted   = "deleted"
	netBirdResultUnchanged = "unchanged"
)

const (
	netBirdDefaultPolicyActionDisable = "disable"
	netBirdDefaultPolicyActionDelete  = "delete"
)

type NetBirdAPI interface {
	ListPeers(ctx context.Context) ([]netbirdapi.Peer, error)
	ListGroups(ctx context.Context) ([]netbirdapi.Group, error)
	CreateGroup(ctx context.Context, input netbirdapi.GroupRequest) (netbirdapi.Group, error)
	UpdateGroup(ctx context.Context, groupID string, input netbirdapi.GroupRequest) (netbirdapi.Group, error)
	DeleteGroup(ctx context.Context, groupID string) error
	ListPolicies(ctx context.Context) ([]netbirdapi.Policy, error)
	CreatePolicy(ctx context.Context, input netbirdapi.PolicyRequest) (netbirdapi.Policy, error)
	UpdatePolicy(ctx context.Context, policyID string, input netbirdapi.PolicyRequest) (netbirdapi.Policy, error)
	DeletePolicy(ctx context.Context, policyID string) error
}

type NetBirdReconcileInput struct {
	Catalog             NetBirdCatalog `json:"catalog"`
	HostPeerID          string         `json:"hostPeerId"`
	AdminPeerIDs        []string       `json:"adminPeerIds"`
	DefaultPolicyAction string         `json:"defaultPolicyAction,omitempty"`
}

type NetBirdReconcileOperation struct {
	Name       string `json:"name"`
	ResourceID string `json:"resourceId,omitempty"`
	Result     string `json:"result"`
}

type NetBirdReconcileResult struct {
	DefaultPolicy    NetBirdReconcileOperation   `json:"defaultPolicy"`
	GroupOperations  []NetBirdReconcileOperation `json:"groupOperations"`
	PolicyOperations []NetBirdReconcileOperation `json:"policyOperations"`
}

func (s *NetBirdService) ReconcileManagedCatalogWithToken(ctx context.Context, apiBaseURL, apiToken string, input NetBirdReconcileInput) (NetBirdReconcileResult, error) {
	client := netbirdapi.NewClient(apiBaseURL, apiToken)
	return s.ReconcileManagedCatalog(ctx, client, input)
}

func (s *NetBirdService) ReconcileManagedCatalog(ctx context.Context, api NetBirdAPI, input NetBirdReconcileInput) (NetBirdReconcileResult, error) {
	if s == nil {
		return NetBirdReconcileResult{}, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable")
	}
	if api == nil {
		return NetBirdReconcileResult{}, errs.New(errs.CodeNetBirdUnavailable, "netbird api client unavailable")
	}

	hostPeerID := strings.TrimSpace(input.HostPeerID)
	adminPeerIDs := normalizeStringList(input.AdminPeerIDs)
	action := normalizeDefaultPolicyAction(input.DefaultPolicyAction)

	targetGroups, err := resolveTargetGroups(input.Catalog.Groups, hostPeerID, adminPeerIDs)
	if err != nil {
		return NetBirdReconcileResult{}, errs.Wrap(errs.CodeNetBirdReconcileFailed, "failed to resolve target netbird groups", err)
	}

	targetPolicies := normalizePolicyPayloads(input.Catalog.Policies)

	groupOps, err := reconcileGroups(ctx, api, targetGroups)
	if err != nil {
		return NetBirdReconcileResult{}, errs.Wrap(errs.CodeNetBirdReconcileFailed, "failed to reconcile netbird groups", err)
	}

	groupsAfter, err := api.ListGroups(ctx)
	if err != nil {
		return NetBirdReconcileResult{}, errs.Wrap(errs.CodeNetBirdReconcileFailed, "failed to list netbird groups after reconcile", err)
	}
	groupIDsByName := groupIDMap(groupsAfter)

	policiesBeforeDefault, err := api.ListPolicies(ctx)
	if err != nil {
		return NetBirdReconcileResult{}, errs.Wrap(errs.CodeNetBirdReconcileFailed, "failed to list netbird policies", err)
	}

	defaultOp, err := applyDefaultPolicyAction(ctx, api, policiesBeforeDefault, action)
	if err != nil {
		return NetBirdReconcileResult{}, errs.Wrap(errs.CodeNetBirdReconcileFailed, "failed to apply netbird default policy action", err)
	}

	policiesCurrent, err := api.ListPolicies(ctx)
	if err != nil {
		return NetBirdReconcileResult{}, errs.Wrap(errs.CodeNetBirdReconcileFailed, "failed to list netbird policies after default handling", err)
	}

	policyOps, err := reconcilePolicies(ctx, api, policiesCurrent, targetPolicies, groupIDsByName)
	if err != nil {
		return NetBirdReconcileResult{}, errs.Wrap(errs.CodeNetBirdReconcileFailed, "failed to reconcile netbird policies", err)
	}

	return NetBirdReconcileResult{
		DefaultPolicy:    defaultOp,
		GroupOperations:  groupOps,
		PolicyOperations: policyOps,
	}, nil
}

func reconcileGroups(ctx context.Context, api NetBirdAPI, target []netbirdapi.GroupRequest) ([]NetBirdReconcileOperation, error) {
	existing, err := api.ListGroups(ctx)
	if err != nil {
		return nil, err
	}

	currentByName := indexGroupsByName(existing)
	targetSet := make(map[string]struct{}, len(target))
	ops := make([]NetBirdReconcileOperation, 0, len(existing)+len(target))

	for _, desired := range target {
		name := strings.TrimSpace(desired.Name)
		if name == "" {
			continue
		}
		targetSet[name] = struct{}{}
		current := currentByName[name]
		if len(current) == 0 {
			created, err := api.CreateGroup(ctx, desired)
			if err != nil {
				return nil, err
			}
			ops = append(ops, NetBirdReconcileOperation{Name: name, ResourceID: strings.TrimSpace(created.ID), Result: netBirdResultCreated})
			continue
		}

		primary := current[0]
		if groupsEqual(primary, desired) {
			ops = append(ops, NetBirdReconcileOperation{Name: name, ResourceID: strings.TrimSpace(primary.ID), Result: netBirdResultUnchanged})
		} else {
			updated, err := api.UpdateGroup(ctx, primary.ID, desired)
			if err != nil {
				return nil, err
			}
			ops = append(ops, NetBirdReconcileOperation{Name: name, ResourceID: strings.TrimSpace(updated.ID), Result: netBirdResultUpdated})
		}

		if len(current) <= 1 {
			continue
		}
		for _, duplicate := range current[1:] {
			if err := api.DeleteGroup(ctx, duplicate.ID); err != nil {
				return nil, err
			}
			ops = append(ops, NetBirdReconcileOperation{Name: name, ResourceID: strings.TrimSpace(duplicate.ID), Result: netBirdResultDeleted})
		}
	}

	stale := staleGroups(existing, targetSet)
	for _, entry := range stale {
		if err := api.DeleteGroup(ctx, entry.ID); err != nil {
			return nil, err
		}
		ops = append(ops, NetBirdReconcileOperation{Name: strings.TrimSpace(entry.Name), ResourceID: strings.TrimSpace(entry.ID), Result: netBirdResultDeleted})
	}

	return ops, nil
}

func reconcilePolicies(
	ctx context.Context,
	api NetBirdAPI,
	existing []netbirdapi.Policy,
	target []NetBirdPolicyPayload,
	groupIDsByName map[string]string,
) ([]NetBirdReconcileOperation, error) {
	currentByName := indexPoliciesByName(existing)
	targetSet := make(map[string]struct{}, len(target))
	ops := make([]NetBirdReconcileOperation, 0, len(existing)+len(target))

	for _, payload := range target {
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			continue
		}
		targetSet[name] = struct{}{}
		desired, err := resolvePolicyRequest(payload, groupIDsByName)
		if err != nil {
			return nil, err
		}

		current := currentByName[name]
		if len(current) == 0 {
			created, err := api.CreatePolicy(ctx, desired)
			if err != nil {
				return nil, err
			}
			ops = append(ops, NetBirdReconcileOperation{Name: name, ResourceID: strings.TrimSpace(created.ID), Result: netBirdResultCreated})
			continue
		}

		primary := current[0]
		if policiesEqual(primary, desired) {
			ops = append(ops, NetBirdReconcileOperation{Name: name, ResourceID: strings.TrimSpace(primary.ID), Result: netBirdResultUnchanged})
		} else {
			updated, err := api.UpdatePolicy(ctx, primary.ID, desired)
			if err != nil {
				return nil, err
			}
			ops = append(ops, NetBirdReconcileOperation{Name: name, ResourceID: strings.TrimSpace(updated.ID), Result: netBirdResultUpdated})
		}

		if len(current) <= 1 {
			continue
		}
		for _, duplicate := range current[1:] {
			if err := api.DeletePolicy(ctx, duplicate.ID); err != nil {
				return nil, err
			}
			ops = append(ops, NetBirdReconcileOperation{Name: name, ResourceID: strings.TrimSpace(duplicate.ID), Result: netBirdResultDeleted})
		}
	}

	stale := stalePolicies(existing, targetSet)
	for _, entry := range stale {
		if err := api.DeletePolicy(ctx, entry.ID); err != nil {
			return nil, err
		}
		ops = append(ops, NetBirdReconcileOperation{Name: strings.TrimSpace(entry.Name), ResourceID: strings.TrimSpace(entry.ID), Result: netBirdResultDeleted})
	}

	return ops, nil
}

func applyDefaultPolicyAction(
	ctx context.Context,
	api NetBirdAPI,
	policies []netbirdapi.Policy,
	action string,
) (NetBirdReconcileOperation, error) {
	operation := NetBirdReconcileOperation{
		Name:   netBirdDefaultPolicyName,
		Result: netBirdResultUnchanged,
	}
	if action == netBirdDefaultPolicyActionNone {
		return operation, nil
	}

	defaults := defaultPolicies(policies)
	if len(defaults) == 0 {
		return operation, nil
	}

	highestResult := netBirdResultUnchanged
	for _, policy := range defaults {
		operation.ResourceID = strings.TrimSpace(policy.ID)
		switch action {
		case netBirdDefaultPolicyActionDelete:
			if err := api.DeletePolicy(ctx, policy.ID); err != nil {
				return NetBirdReconcileOperation{}, err
			}
			highestResult = prioritizeResult(highestResult, netBirdResultDeleted)
		default:
			if !policy.Enabled {
				continue
			}
			request := policyToRequest(policy)
			request.Enabled = false
			updated, err := api.UpdatePolicy(ctx, policy.ID, request)
			if err != nil {
				return NetBirdReconcileOperation{}, err
			}
			operation.ResourceID = strings.TrimSpace(updated.ID)
			highestResult = prioritizeResult(highestResult, netBirdResultUpdated)
		}
	}

	operation.Result = highestResult
	return operation, nil
}

func resolveTargetGroups(catalog []NetBirdGroupPayload, hostPeerID string, adminPeerIDs []string) ([]netbirdapi.GroupRequest, error) {
	normalized := normalizeGroupPayloads(catalog)
	result := make([]netbirdapi.GroupRequest, 0, len(normalized))

	for _, payload := range normalized {
		peers, err := resolveGroupPeers(payload.Peers, hostPeerID, adminPeerIDs)
		if err != nil {
			return nil, fmt.Errorf("group %s: %w", strings.TrimSpace(payload.Name), err)
		}
		result = append(result, netbirdapi.GroupRequest{
			Name:  strings.TrimSpace(payload.Name),
			Peers: peers,
		})
	}

	return result, nil
}

func resolveGroupPeers(rawPeers []string, hostPeerID string, adminPeerIDs []string) ([]string, error) {
	hostPeerID = strings.TrimSpace(hostPeerID)
	admins := normalizeStringList(adminPeerIDs)
	resolved := make([]string, 0, len(rawPeers)+len(admins))
	adminExpanded := false

	for _, entry := range rawPeers {
		peerRef := strings.TrimSpace(entry)
		if peerRef == "" {
			continue
		}
		switch peerRef {
		case netBirdHostPeerPlaceholder:
			if hostPeerID == "" {
				return nil, fmt.Errorf("host peer id is required")
			}
			resolved = append(resolved, hostPeerID)
		case netBirdAdminsPeerPlaceholderA, netBirdAdminsPeerPlaceholderB:
			if len(admins) == 0 {
				return nil, fmt.Errorf("at least one admin peer id is required")
			}
			if adminExpanded {
				continue
			}
			resolved = append(resolved, admins...)
			adminExpanded = true
		default:
			resolved = append(resolved, peerRef)
		}
	}

	resolved = normalizeStringList(resolved)
	if len(resolved) == 0 {
		return nil, fmt.Errorf("resolved peers cannot be empty")
	}
	return resolved, nil
}

func resolvePolicyRequest(payload NetBirdPolicyPayload, groupIDsByName map[string]string) (netbirdapi.PolicyRequest, error) {
	request := netbirdapi.PolicyRequest{
		Name:        strings.TrimSpace(payload.Name),
		Description: strings.TrimSpace(payload.Description),
		Enabled:     payload.Enabled,
		Rules:       make([]netbirdapi.PolicyRule, 0, len(payload.Rules)),
	}

	for _, rawRule := range payload.Rules {
		sources, err := resolvePolicyReferences(rawRule.Sources, groupIDsByName)
		if err != nil {
			return netbirdapi.PolicyRequest{}, fmt.Errorf("policy %s rule %s sources: %w", request.Name, strings.TrimSpace(rawRule.Name), err)
		}
		destinations, err := resolvePolicyReferences(rawRule.Destinations, groupIDsByName)
		if err != nil {
			return netbirdapi.PolicyRequest{}, fmt.Errorf("policy %s rule %s destinations: %w", request.Name, strings.TrimSpace(rawRule.Name), err)
		}
		request.Rules = append(request.Rules, netbirdapi.PolicyRule{
			Name:          strings.TrimSpace(rawRule.Name),
			Description:   strings.TrimSpace(rawRule.Description),
			Enabled:       rawRule.Enabled,
			Action:        strings.TrimSpace(rawRule.Action),
			Bidirectional: rawRule.Bidirectional,
			Protocol:      strings.TrimSpace(rawRule.Protocol),
			Ports:         normalizeStringList(rawRule.Ports),
			Sources:       sources,
			Destinations:  destinations,
		})
	}

	sortPolicyRequest(&request)
	return request, nil
}

func resolvePolicyReferences(references []string, groupIDsByName map[string]string) ([]string, error) {
	resolved := make([]string, 0, len(references))
	for _, reference := range references {
		value := strings.TrimSpace(reference)
		if value == "" {
			continue
		}
		groupName, ok, err := policyPlaceholderToGroupName(value)
		if err != nil {
			return nil, err
		}
		if ok {
			groupID := strings.TrimSpace(groupIDsByName[groupName])
			if groupID == "" {
				return nil, fmt.Errorf("group id for %q not found", groupName)
			}
			resolved = append(resolved, groupID)
			continue
		}
		resolved = append(resolved, value)
	}
	resolved = normalizeStringList(resolved)
	if len(resolved) == 0 {
		return nil, fmt.Errorf("resolved references cannot be empty")
	}
	return resolved, nil
}

func policyPlaceholderToGroupName(reference string) (string, bool, error) {
	switch reference {
	case netBirdGroupIDAdmins:
		return netBirdGroupAdminsName, true, nil
	case netBirdGroupIDPanel:
		return netBirdGroupPanelName, true, nil
	}

	const prefix = "${GROUP_ID_GUNGNR_PROJECT_"
	const suffix = "}"
	if !strings.HasPrefix(reference, prefix) || !strings.HasSuffix(reference, suffix) {
		return "", false, nil
	}
	rawID := strings.TrimSuffix(strings.TrimPrefix(reference, prefix), suffix)
	if rawID == "" {
		return "", false, fmt.Errorf("invalid project group placeholder %q", reference)
	}
	parsed, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		return "", false, fmt.Errorf("invalid project group placeholder %q", reference)
	}
	return netBirdProjectGroupName(uint(parsed)), true, nil
}

func groupsEqual(existing netbirdapi.Group, desired netbirdapi.GroupRequest) bool {
	if strings.TrimSpace(existing.Name) != strings.TrimSpace(desired.Name) {
		return false
	}
	a := normalizeStringList(existing.Peers)
	b := normalizeStringList(desired.Peers)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func policiesEqual(existing netbirdapi.Policy, desired netbirdapi.PolicyRequest) bool {
	left := normalizePolicyRequest(policyToRequest(existing))
	right := normalizePolicyRequest(desired)
	if left.Name != right.Name || left.Description != right.Description || left.Enabled != right.Enabled {
		return false
	}
	if len(left.Rules) != len(right.Rules) {
		return false
	}
	for i := range left.Rules {
		if !policyRulesEqual(left.Rules[i], right.Rules[i]) {
			return false
		}
	}
	return true
}

func policyRulesEqual(a netbirdapi.PolicyRule, b netbirdapi.PolicyRule) bool {
	if a.Name != b.Name || a.Description != b.Description || a.Enabled != b.Enabled || a.Action != b.Action || a.Bidirectional != b.Bidirectional || a.Protocol != b.Protocol {
		return false
	}
	if !stringSlicesEqual(a.Ports, b.Ports) {
		return false
	}
	if !stringSlicesEqual(a.Sources, b.Sources) {
		return false
	}
	if !stringSlicesEqual(a.Destinations, b.Destinations) {
		return false
	}
	return true
}

func stringSlicesEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortPolicyRequest(request *netbirdapi.PolicyRequest) {
	if request == nil {
		return
	}
	*request = normalizePolicyRequest(*request)
}

func normalizePolicyRequest(request netbirdapi.PolicyRequest) netbirdapi.PolicyRequest {
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	if request.Rules == nil {
		request.Rules = []netbirdapi.PolicyRule{}
	}
	for i := range request.Rules {
		rule := request.Rules[i]
		rule.Name = strings.TrimSpace(rule.Name)
		rule.Description = strings.TrimSpace(rule.Description)
		rule.Action = strings.TrimSpace(rule.Action)
		rule.Protocol = strings.TrimSpace(rule.Protocol)
		rule.Ports = normalizeStringList(rule.Ports)
		rule.Sources = normalizeStringList(rule.Sources)
		rule.Destinations = normalizeStringList(rule.Destinations)
		request.Rules[i] = rule
	}
	sort.Slice(request.Rules, func(i, j int) bool {
		return policyRuleSortKey(request.Rules[i]) < policyRuleSortKey(request.Rules[j])
	})
	return request
}

func policyRuleSortKey(rule netbirdapi.PolicyRule) string {
	return strings.Join([]string{
		rule.Name,
		rule.Action,
		rule.Protocol,
		strings.Join(rule.Ports, ","),
		strings.Join(rule.Sources, ","),
		strings.Join(rule.Destinations, ","),
	}, "|")
}

func policyToRequest(policy netbirdapi.Policy) netbirdapi.PolicyRequest {
	request := netbirdapi.PolicyRequest{
		Name:        strings.TrimSpace(policy.Name),
		Description: strings.TrimSpace(policy.Description),
		Enabled:     policy.Enabled,
		Rules:       make([]netbirdapi.PolicyRule, 0, len(policy.Rules)),
	}
	for _, rule := range policy.Rules {
		request.Rules = append(request.Rules, netbirdapi.PolicyRule{
			Name:          strings.TrimSpace(rule.Name),
			Description:   strings.TrimSpace(rule.Description),
			Enabled:       rule.Enabled,
			Action:        strings.TrimSpace(rule.Action),
			Bidirectional: rule.Bidirectional,
			Protocol:      strings.TrimSpace(rule.Protocol),
			Ports:         normalizeStringList(rule.Ports),
			Sources:       normalizeStringList(rule.Sources),
			Destinations:  normalizeStringList(rule.Destinations),
		})
	}
	return normalizePolicyRequest(request)
}

func normalizeGroupPayloads(catalog []NetBirdGroupPayload) []NetBirdGroupPayload {
	out := make([]NetBirdGroupPayload, 0, len(catalog))
	for _, group := range catalog {
		name := strings.TrimSpace(group.Name)
		if name == "" {
			continue
		}
		out = append(out, NetBirdGroupPayload{
			Name:  name,
			Peers: normalizeStringList(group.Peers),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func normalizePolicyPayloads(catalog []NetBirdPolicyPayload) []NetBirdPolicyPayload {
	out := make([]NetBirdPolicyPayload, 0, len(catalog))
	for _, policy := range catalog {
		name := strings.TrimSpace(policy.Name)
		if name == "" {
			continue
		}
		rules := make([]NetBirdPolicyRuleSpec, 0, len(policy.Rules))
		for _, rule := range policy.Rules {
			rules = append(rules, NetBirdPolicyRuleSpec{
				Name:          strings.TrimSpace(rule.Name),
				Description:   strings.TrimSpace(rule.Description),
				Enabled:       rule.Enabled,
				Action:        strings.TrimSpace(rule.Action),
				Bidirectional: rule.Bidirectional,
				Protocol:      strings.TrimSpace(rule.Protocol),
				Ports:         normalizeStringList(rule.Ports),
				Sources:       normalizeStringList(rule.Sources),
				Destinations:  normalizeStringList(rule.Destinations),
			})
		}
		sort.Slice(rules, func(i, j int) bool {
			if rules[i].Name == rules[j].Name {
				return rules[i].Action < rules[j].Action
			}
			return rules[i].Name < rules[j].Name
		})
		out = append(out, NetBirdPolicyPayload{
			Name:        name,
			Description: strings.TrimSpace(policy.Description),
			Enabled:     policy.Enabled,
			Rules:       rules,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func normalizeStringList(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeDefaultPolicyAction(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case netBirdDefaultPolicyActionDelete:
		return netBirdDefaultPolicyActionDelete
	case netBirdDefaultPolicyActionNone:
		return netBirdDefaultPolicyActionNone
	default:
		return netBirdDefaultPolicyActionDisable
	}
}

func indexGroupsByName(groups []netbirdapi.Group) map[string][]netbirdapi.Group {
	sorted := append([]netbirdapi.Group(nil), groups...)
	sort.Slice(sorted, func(i, j int) bool {
		leftName := strings.TrimSpace(sorted[i].Name)
		rightName := strings.TrimSpace(sorted[j].Name)
		if leftName == rightName {
			return strings.TrimSpace(sorted[i].ID) < strings.TrimSpace(sorted[j].ID)
		}
		return leftName < rightName
	})
	indexed := make(map[string][]netbirdapi.Group, len(sorted))
	for _, group := range sorted {
		name := strings.TrimSpace(group.Name)
		if name == "" {
			continue
		}
		indexed[name] = append(indexed[name], group)
	}
	return indexed
}

func groupIDMap(groups []netbirdapi.Group) map[string]string {
	indexed := indexGroupsByName(groups)
	result := make(map[string]string, len(indexed))
	for name, entries := range indexed {
		if len(entries) == 0 {
			continue
		}
		result[name] = strings.TrimSpace(entries[0].ID)
	}
	return result
}

func indexPoliciesByName(policies []netbirdapi.Policy) map[string][]netbirdapi.Policy {
	sorted := append([]netbirdapi.Policy(nil), policies...)
	sort.Slice(sorted, func(i, j int) bool {
		leftName := strings.TrimSpace(sorted[i].Name)
		rightName := strings.TrimSpace(sorted[j].Name)
		if leftName == rightName {
			return strings.TrimSpace(sorted[i].ID) < strings.TrimSpace(sorted[j].ID)
		}
		return leftName < rightName
	})
	indexed := make(map[string][]netbirdapi.Policy, len(sorted))
	for _, policy := range sorted {
		name := strings.TrimSpace(policy.Name)
		if name == "" {
			continue
		}
		indexed[name] = append(indexed[name], policy)
	}
	return indexed
}

func staleGroups(groups []netbirdapi.Group, keep map[string]struct{}) []netbirdapi.Group {
	result := make([]netbirdapi.Group, 0)
	for _, group := range groups {
		name := strings.TrimSpace(group.Name)
		if !strings.HasPrefix(name, "gungnr-") {
			continue
		}
		if _, exists := keep[name]; exists {
			continue
		}
		result = append(result, group)
	}
	sort.Slice(result, func(i, j int) bool {
		leftName := strings.TrimSpace(result[i].Name)
		rightName := strings.TrimSpace(result[j].Name)
		if leftName == rightName {
			return strings.TrimSpace(result[i].ID) < strings.TrimSpace(result[j].ID)
		}
		return leftName < rightName
	})
	return result
}

func stalePolicies(policies []netbirdapi.Policy, keep map[string]struct{}) []netbirdapi.Policy {
	result := make([]netbirdapi.Policy, 0)
	for _, policy := range policies {
		name := strings.TrimSpace(policy.Name)
		if !strings.HasPrefix(name, "gungnr-") {
			continue
		}
		if _, exists := keep[name]; exists {
			continue
		}
		result = append(result, policy)
	}
	sort.Slice(result, func(i, j int) bool {
		leftName := strings.TrimSpace(result[i].Name)
		rightName := strings.TrimSpace(result[j].Name)
		if leftName == rightName {
			return strings.TrimSpace(result[i].ID) < strings.TrimSpace(result[j].ID)
		}
		return leftName < rightName
	})
	return result
}

func defaultPolicies(policies []netbirdapi.Policy) []netbirdapi.Policy {
	result := make([]netbirdapi.Policy, 0)
	for _, policy := range policies {
		if strings.EqualFold(strings.TrimSpace(policy.Name), netBirdDefaultPolicyName) {
			result = append(result, policy)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.TrimSpace(result[i].ID) < strings.TrimSpace(result[j].ID)
	})
	return result
}

func prioritizeResult(current, candidate string) string {
	rank := map[string]int{
		netBirdResultUnchanged: 0,
		netBirdResultUpdated:   1,
		netBirdResultDeleted:   2,
		netBirdResultCreated:   3,
	}
	if rank[candidate] > rank[current] {
		return candidate
	}
	return current
}

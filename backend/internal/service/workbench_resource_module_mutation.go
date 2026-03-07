package service

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go-notes/internal/errs"
)

const (
	workbenchResourceMutationActionSet   = "set"
	workbenchResourceMutationActionClear = "clear"

	workbenchModuleMutationActionAdd    = "add"
	workbenchModuleMutationActionRemove = "remove"

	workbenchMutationIssueClassSchema   = "schema"
	workbenchMutationIssueClassValue    = "value"
	workbenchMutationIssueClassConflict = "conflict"

	workbenchResourceFieldLimitCPUs         = "limitCpus"
	workbenchResourceFieldLimitMemory       = "limitMemory"
	workbenchResourceFieldReservationCPUs   = "reservationCpus"
	workbenchResourceFieldReservationMemory = "reservationMemory"
)

var workbenchResourceMemoryPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:[kKmMgGtTpPeE](?:[iI]?[bB]?)?)?$`)

type WorkbenchResourceSelector struct {
	ServiceName string `json:"serviceName"`
}

type WorkbenchResourceMutationRequest struct {
	Selector          WorkbenchResourceSelector `json:"selector"`
	Action            string                    `json:"action"`
	LimitCPUs         *string                   `json:"limitCpus,omitempty"`
	LimitMemory       *string                   `json:"limitMemory,omitempty"`
	ReservationCPUs   *string                   `json:"reservationCpus,omitempty"`
	ReservationMemory *string                   `json:"reservationMemory,omitempty"`
	ClearFields       []string                  `json:"clearFields,omitempty"`
}

type WorkbenchResourceMutationSummary struct {
	Changed          bool                      `json:"changed"`
	Action           string                    `json:"action"`
	Selector         WorkbenchResourceSelector `json:"selector"`
	UpdatedFields    []string                  `json:"updatedFields,omitempty"`
	ClearedFields    []string                  `json:"clearedFields,omitempty"`
	PreviousResource *WorkbenchComposeResource `json:"previousResource,omitempty"`
	CurrentResource  *WorkbenchComposeResource `json:"currentResource,omitempty"`
}

type WorkbenchModuleSelector struct {
	ServiceName string `json:"serviceName"`
	ModuleType  string `json:"moduleType"`
}

type WorkbenchModuleMutationRequest struct {
	Selector WorkbenchModuleSelector `json:"selector"`
	Action   string                  `json:"action"`
}

type WorkbenchModuleMutationSummary struct {
	Changed       bool                    `json:"changed"`
	Action        string                  `json:"action"`
	Selector      WorkbenchModuleSelector `json:"selector"`
	PreviousCount int                     `json:"previousCount"`
	CurrentCount  int                     `json:"currentCount"`
}

type WorkbenchMutationIssue struct {
	Class      string `json:"class"`
	Code       string `json:"code"`
	Path       string `json:"path"`
	Message    string `json:"message"`
	EntryKey   string `json:"entryKey,omitempty"`
	Service    string `json:"service,omitempty"`
	Field      string `json:"field,omitempty"`
	ModuleType string `json:"moduleType,omitempty"`
	Action     string `json:"action,omitempty"`
}

func (s *WorkbenchService) MutateStoredSnapshotResource(
	ctx context.Context,
	projectName string,
	input WorkbenchResourceMutationRequest,
) (WorkbenchStackSnapshot, WorkbenchResourceMutationSummary, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchResourceMutationSummary{}, err
	}

	normalizedInput, issues := normalizeWorkbenchResourceMutationRequest(input)
	summary := WorkbenchResourceMutationSummary{
		Action:        normalizedInput.Action,
		Selector:      normalizedInput.Selector,
		UpdatedFields: []string{},
		ClearedFields: []string{},
	}
	if len(issues) > 0 {
		return WorkbenchStackSnapshot{}, summary, workbenchResourceMutationValidationError(WorkbenchStackSnapshot{}, summary, issues)
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchResourceMutationSummary{}, err
	}
	defer release()

	snapshot, exists, err := s.loadStoredWorkbenchSnapshot(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchResourceMutationSummary{}, err
	}
	if !exists {
		return WorkbenchStackSnapshot{}, WorkbenchResourceMutationSummary{}, errs.WithDetails(
			errs.New(errs.CodeWorkbenchSourceNotFound, fmt.Sprintf("workbench snapshot not found for project %q", normalizedProject)),
			map[string]any{
				"project": normalizedProject,
			},
		)
	}

	mutated, mutationSummary, mutationIssues := mutateWorkbenchSnapshotResource(snapshot, normalizedInput)
	if len(mutationIssues) > 0 {
		return mutated, mutationSummary, workbenchResourceMutationValidationError(mutated, mutationSummary, mutationIssues)
	}
	if !mutationSummary.Changed {
		return mutated, mutationSummary, nil
	}

	if mutated.Revision <= 0 {
		mutated.Revision = 1
	}
	mutated.Revision++
	if err := s.saveWorkbenchSnapshot(ctx, normalizedProject, mutated); err != nil {
		return mutated, mutationSummary, err
	}
	return mutated, mutationSummary, nil
}

func (s *WorkbenchService) MutateStoredSnapshotModule(
	ctx context.Context,
	projectName string,
	input WorkbenchModuleMutationRequest,
) (WorkbenchStackSnapshot, WorkbenchModuleMutationSummary, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchModuleMutationSummary{}, err
	}

	normalizedInput, issues := normalizeWorkbenchModuleMutationRequest(input)
	summary := WorkbenchModuleMutationSummary{
		Action:   normalizedInput.Action,
		Selector: normalizedInput.Selector,
	}
	if len(issues) > 0 {
		return WorkbenchStackSnapshot{}, summary, workbenchModuleMutationValidationError(WorkbenchStackSnapshot{}, summary, issues)
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchModuleMutationSummary{}, err
	}
	defer release()

	snapshot, exists, err := s.loadStoredWorkbenchSnapshot(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchModuleMutationSummary{}, err
	}
	if !exists {
		return WorkbenchStackSnapshot{}, WorkbenchModuleMutationSummary{}, errs.WithDetails(
			errs.New(errs.CodeWorkbenchSourceNotFound, fmt.Sprintf("workbench snapshot not found for project %q", normalizedProject)),
			map[string]any{
				"project": normalizedProject,
			},
		)
	}

	mutated, mutationSummary, mutationIssues := mutateWorkbenchSnapshotModule(snapshot, normalizedInput)
	if len(mutationIssues) > 0 {
		return mutated, mutationSummary, workbenchModuleMutationValidationError(mutated, mutationSummary, mutationIssues)
	}
	if !mutationSummary.Changed {
		return mutated, mutationSummary, nil
	}

	if mutated.Revision <= 0 {
		mutated.Revision = 1
	}
	mutated.Revision++
	if err := s.saveWorkbenchSnapshot(ctx, normalizedProject, mutated); err != nil {
		return mutated, mutationSummary, err
	}
	return mutated, mutationSummary, nil
}

func normalizeWorkbenchResourceMutationRequest(
	input WorkbenchResourceMutationRequest,
) (WorkbenchResourceMutationRequest, []WorkbenchMutationIssue) {
	normalized := input
	issues := []WorkbenchMutationIssue{}

	normalized.Selector.ServiceName = strings.TrimSpace(input.Selector.ServiceName)
	normalized.Action = strings.ToLower(strings.TrimSpace(input.Action))
	normalized.LimitCPUs = workbenchNormalizedStringPtr(input.LimitCPUs)
	normalized.LimitMemory = workbenchNormalizedStringPtr(input.LimitMemory)
	normalized.ReservationCPUs = workbenchNormalizedStringPtr(input.ReservationCPUs)
	normalized.ReservationMemory = workbenchNormalizedStringPtr(input.ReservationMemory)

	if normalized.Selector.ServiceName == "" {
		issues = append(issues, WorkbenchMutationIssue{
			Class:   workbenchMutationIssueClassSchema,
			Code:    "WB-RESOURCE-SELECTOR-SERVICE-REQUIRED",
			Path:    "$.selector.serviceName",
			Message: "selector.serviceName is required",
			Action:  normalized.Action,
		})
	}

	switch normalized.Action {
	case workbenchResourceMutationActionSet:
		if len(input.ClearFields) > 0 {
			issues = append(issues, WorkbenchMutationIssue{
				Class:   workbenchMutationIssueClassSchema,
				Code:    "WB-RESOURCE-CLEAR-FIELDS-UNEXPECTED",
				Path:    "$.clearFields",
				Message: "clearFields must be omitted when action is set",
				Action:  normalized.Action,
			})
		}
		normalized.ClearFields = []string{}
		provided := 0
		issues = append(issues, validateWorkbenchResourceFieldValue(
			normalized.LimitCPUs,
			workbenchResourceFieldLimitCPUs,
			"$.limitCpus",
			normalized.Action,
			&provided,
		)...)
		issues = append(issues, validateWorkbenchResourceFieldValue(
			normalized.LimitMemory,
			workbenchResourceFieldLimitMemory,
			"$.limitMemory",
			normalized.Action,
			&provided,
		)...)
		issues = append(issues, validateWorkbenchResourceFieldValue(
			normalized.ReservationCPUs,
			workbenchResourceFieldReservationCPUs,
			"$.reservationCpus",
			normalized.Action,
			&provided,
		)...)
		issues = append(issues, validateWorkbenchResourceFieldValue(
			normalized.ReservationMemory,
			workbenchResourceFieldReservationMemory,
			"$.reservationMemory",
			normalized.Action,
			&provided,
		)...)
		if provided == 0 {
			issues = append(issues, WorkbenchMutationIssue{
				Class:   workbenchMutationIssueClassSchema,
				Code:    "WB-RESOURCE-SET-REQUIRED",
				Path:    "$",
				Message: "at least one resource field is required when action is set",
				Action:  normalized.Action,
			})
		}
	case workbenchResourceMutationActionClear:
		if normalized.LimitCPUs != nil {
			issues = append(issues, workbenchResourceUnexpectedValueIssue(workbenchResourceFieldLimitCPUs, "$.limitCpus", normalized.Action))
		}
		if normalized.LimitMemory != nil {
			issues = append(issues, workbenchResourceUnexpectedValueIssue(workbenchResourceFieldLimitMemory, "$.limitMemory", normalized.Action))
		}
		if normalized.ReservationCPUs != nil {
			issues = append(issues, workbenchResourceUnexpectedValueIssue(workbenchResourceFieldReservationCPUs, "$.reservationCpus", normalized.Action))
		}
		if normalized.ReservationMemory != nil {
			issues = append(issues, workbenchResourceUnexpectedValueIssue(workbenchResourceFieldReservationMemory, "$.reservationMemory", normalized.Action))
		}

		normalizedClear, clearIssues := normalizeWorkbenchResourceClearFields(input.ClearFields, normalized.Action)
		normalized.ClearFields = normalizedClear
		issues = append(issues, clearIssues...)
	default:
		issues = append(issues, WorkbenchMutationIssue{
			Class:   workbenchMutationIssueClassSchema,
			Code:    "WB-RESOURCE-ACTION-INVALID",
			Path:    "$.action",
			Message: fmt.Sprintf("invalid action %q; expected %q or %q", input.Action, workbenchResourceMutationActionSet, workbenchResourceMutationActionClear),
			Action:  normalized.Action,
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchMutationIssueLess(issues[i], issues[j])
		})
	}
	return normalized, issues
}

func normalizeWorkbenchModuleMutationRequest(
	input WorkbenchModuleMutationRequest,
) (WorkbenchModuleMutationRequest, []WorkbenchMutationIssue) {
	normalized := input
	issues := []WorkbenchMutationIssue{}

	normalized.Selector.ServiceName = strings.TrimSpace(input.Selector.ServiceName)
	normalized.Selector.ModuleType = strings.ToLower(strings.TrimSpace(input.Selector.ModuleType))
	normalized.Action = strings.ToLower(strings.TrimSpace(input.Action))

	if normalized.Selector.ServiceName == "" {
		issues = append(issues, WorkbenchMutationIssue{
			Class:   workbenchMutationIssueClassSchema,
			Code:    "WB-MODULE-SELECTOR-SERVICE-REQUIRED",
			Path:    "$.selector.serviceName",
			Message: "selector.serviceName is required",
			Action:  normalized.Action,
		})
	}
	if normalized.Selector.ModuleType == "" {
		issues = append(issues, WorkbenchMutationIssue{
			Class:   workbenchMutationIssueClassSchema,
			Code:    "WB-MODULE-SELECTOR-TYPE-REQUIRED",
			Path:    "$.selector.moduleType",
			Message: "selector.moduleType is required",
			Service: normalized.Selector.ServiceName,
			Action:  normalized.Action,
		})
	}

	switch normalized.Action {
	case workbenchModuleMutationActionAdd, workbenchModuleMutationActionRemove:
	default:
		issues = append(issues, WorkbenchMutationIssue{
			Class:      workbenchMutationIssueClassSchema,
			Code:       "WB-MODULE-ACTION-INVALID",
			Path:       "$.action",
			Message:    fmt.Sprintf("invalid action %q; expected %q or %q", input.Action, workbenchModuleMutationActionAdd, workbenchModuleMutationActionRemove),
			Service:    normalized.Selector.ServiceName,
			ModuleType: normalized.Selector.ModuleType,
			Action:     normalized.Action,
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchMutationIssueLess(issues[i], issues[j])
		})
	}
	return normalized, issues
}

func mutateWorkbenchSnapshotResource(
	snapshot WorkbenchStackSnapshot,
	input WorkbenchResourceMutationRequest,
) (WorkbenchStackSnapshot, WorkbenchResourceMutationSummary, []WorkbenchMutationIssue) {
	normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
	beforeResources := append([]WorkbenchComposeResource(nil), normalizedSnapshot.Resources...)

	summary := WorkbenchResourceMutationSummary{
		Action:        input.Action,
		Selector:      input.Selector,
		UpdatedFields: []string{},
		ClearedFields: []string{},
	}

	if !workbenchSnapshotHasService(normalizedSnapshot.Services, input.Selector.ServiceName) {
		return normalizedSnapshot, summary, []WorkbenchMutationIssue{
			{
				Class:   workbenchMutationIssueClassSchema,
				Code:    "WB-RESOURCE-SELECTOR-NOT-FOUND",
				Path:    "$.selector.serviceName",
				Message: fmt.Sprintf("selector did not match any stored service %q", input.Selector.ServiceName),
				Service: input.Selector.ServiceName,
				Action:  input.Action,
			},
		}
	}

	resourceIndex := workbenchFindResourceIndex(normalizedSnapshot.Resources, input.Selector.ServiceName)
	current := WorkbenchComposeResource{
		ServiceName: input.Selector.ServiceName,
	}
	if resourceIndex >= 0 {
		current = normalizeWorkbenchComposeResource(normalizedSnapshot.Resources[resourceIndex])
		summary.PreviousResource = cloneWorkbenchResource(current)
	}

	issues := []WorkbenchMutationIssue{}
	switch input.Action {
	case workbenchResourceMutationActionSet:
		applyField := func(field string, value *string) {
			if value == nil {
				return
			}
			currentValue := workbenchResourceFieldValue(current, field)
			if currentValue == *value {
				return
			}
			workbenchSetResourceField(&current, field, *value)
			summary.UpdatedFields = append(summary.UpdatedFields, field)
		}
		applyField(workbenchResourceFieldLimitCPUs, input.LimitCPUs)
		applyField(workbenchResourceFieldLimitMemory, input.LimitMemory)
		applyField(workbenchResourceFieldReservationCPUs, input.ReservationCPUs)
		applyField(workbenchResourceFieldReservationMemory, input.ReservationMemory)
	case workbenchResourceMutationActionClear:
		for _, field := range input.ClearFields {
			currentValue := workbenchResourceFieldValue(current, field)
			if currentValue == "" {
				continue
			}
			workbenchSetResourceField(&current, field, "")
			summary.ClearedFields = append(summary.ClearedFields, field)
		}
	default:
		issues = append(issues, WorkbenchMutationIssue{
			Class:   workbenchMutationIssueClassSchema,
			Code:    "WB-RESOURCE-ACTION-INVALID",
			Path:    "$.action",
			Message: fmt.Sprintf("invalid action %q", input.Action),
			Service: input.Selector.ServiceName,
			Action:  input.Action,
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchMutationIssueLess(issues[i], issues[j])
		})
		return normalizedSnapshot, summary, issues
	}

	workbenchSortResourceFields(summary.UpdatedFields)
	workbenchSortResourceFields(summary.ClearedFields)

	next := normalizedSnapshot
	current = normalizeWorkbenchComposeResource(current)
	if workbenchIsEmptyResource(current) {
		if resourceIndex >= 0 {
			next.Resources = append(next.Resources[:resourceIndex], next.Resources[resourceIndex+1:]...)
		}
		summary.CurrentResource = nil
	} else {
		if resourceIndex >= 0 {
			next.Resources[resourceIndex] = current
		} else {
			next.Resources = append(next.Resources, current)
		}
		summary.CurrentResource = cloneWorkbenchResource(current)
	}

	next = normalizeWorkbenchStackSnapshot(next)
	summary.Changed = !reflect.DeepEqual(beforeResources, next.Resources)
	if !summary.Changed {
		if existingIndex := workbenchFindResourceIndex(next.Resources, input.Selector.ServiceName); existingIndex >= 0 {
			summary.CurrentResource = cloneWorkbenchResource(next.Resources[existingIndex])
		} else {
			summary.CurrentResource = nil
		}
	}
	return next, summary, nil
}

func mutateWorkbenchSnapshotModule(
	snapshot WorkbenchStackSnapshot,
	input WorkbenchModuleMutationRequest,
) (WorkbenchStackSnapshot, WorkbenchModuleMutationSummary, []WorkbenchMutationIssue) {
	normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
	beforeModules := append([]WorkbenchStackModule(nil), normalizedSnapshot.Modules...)

	summary := WorkbenchModuleMutationSummary{
		Action:   input.Action,
		Selector: input.Selector,
	}

	issues := []WorkbenchMutationIssue{}
	if !workbenchIsSupportedModuleType(input.Selector.ModuleType) {
		issues = append(issues, WorkbenchMutationIssue{
			Class:      workbenchMutationIssueClassSchema,
			Code:       "WB-MODULE-TYPE-UNSUPPORTED",
			Path:       "$.selector.moduleType",
			Message:    fmt.Sprintf("module type %q is not supported", input.Selector.ModuleType),
			Service:    input.Selector.ServiceName,
			ModuleType: input.Selector.ModuleType,
			Action:     input.Action,
		})
	}
	if !workbenchSnapshotHasService(normalizedSnapshot.Services, input.Selector.ServiceName) {
		issues = append(issues, WorkbenchMutationIssue{
			Class:      workbenchMutationIssueClassSchema,
			Code:       "WB-MODULE-TARGET-UNSUPPORTED",
			Path:       "$.selector.serviceName",
			Message:    fmt.Sprintf("module target service %q is not present in snapshot", input.Selector.ServiceName),
			Service:    input.Selector.ServiceName,
			ModuleType: input.Selector.ModuleType,
			Action:     input.Action,
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchMutationIssueLess(issues[i], issues[j])
		})
		return normalizedSnapshot, summary, issues
	}

	target := WorkbenchStackModule{
		ModuleType:  input.Selector.ModuleType,
		ServiceName: input.Selector.ServiceName,
	}
	previousCount := workbenchCountMatchingModules(normalizedSnapshot.Modules, target)
	summary.PreviousCount = previousCount

	next := normalizedSnapshot
	switch input.Action {
	case workbenchModuleMutationActionAdd:
		if previousCount > 0 {
			issues = append(issues, WorkbenchMutationIssue{
				Class:      workbenchMutationIssueClassConflict,
				Code:       "WB-MODULE-DUPLICATE",
				Path:       "$.selector",
				Message:    fmt.Sprintf("module %q is already enabled for service %q", target.ModuleType, target.ServiceName),
				Service:    target.ServiceName,
				ModuleType: target.ModuleType,
				Action:     input.Action,
			})
		} else {
			next.Modules = append(next.Modules, target)
		}
	case workbenchModuleMutationActionRemove:
		filtered := make([]WorkbenchStackModule, 0, len(next.Modules))
		for _, module := range next.Modules {
			if workbenchModuleMatches(module, target) {
				continue
			}
			filtered = append(filtered, module)
		}
		next.Modules = filtered
	default:
		issues = append(issues, WorkbenchMutationIssue{
			Class:      workbenchMutationIssueClassSchema,
			Code:       "WB-MODULE-ACTION-INVALID",
			Path:       "$.action",
			Message:    fmt.Sprintf("invalid action %q", input.Action),
			Service:    target.ServiceName,
			ModuleType: target.ModuleType,
			Action:     input.Action,
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchMutationIssueLess(issues[i], issues[j])
		})
		return normalizedSnapshot, summary, issues
	}

	next = normalizeWorkbenchStackSnapshot(next)
	summary.CurrentCount = workbenchCountMatchingModules(next.Modules, target)
	summary.Changed = !reflect.DeepEqual(beforeModules, next.Modules)
	return next, summary, nil
}

func validateWorkbenchResourceFieldValue(
	value *string,
	field string,
	path string,
	action string,
	provided *int,
) []WorkbenchMutationIssue {
	if value == nil {
		return nil
	}
	*provided++
	issues := []WorkbenchMutationIssue{}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		issues = append(issues, WorkbenchMutationIssue{
			Class:   workbenchMutationIssueClassSchema,
			Code:    workbenchResourceFieldCode(field, "EMPTY"),
			Path:    path,
			Message: fmt.Sprintf("%s cannot be empty when action is set; use clear action instead", field),
			Field:   field,
			Action:  action,
		})
		return issues
	}

	valid := false
	switch field {
	case workbenchResourceFieldLimitCPUs, workbenchResourceFieldReservationCPUs:
		valid = isWorkbenchCPUValue(trimmed)
	case workbenchResourceFieldLimitMemory, workbenchResourceFieldReservationMemory:
		valid = isWorkbenchMemoryValue(trimmed)
	}
	if valid {
		return issues
	}

	issues = append(issues, WorkbenchMutationIssue{
		Class:   workbenchMutationIssueClassValue,
		Code:    workbenchResourceFieldCode(field, "INVALID"),
		Path:    path,
		Message: fmt.Sprintf("%s value %q is invalid", field, trimmed),
		Field:   field,
		Action:  action,
	})
	return issues
}

func workbenchResourceUnexpectedValueIssue(field, path, action string) WorkbenchMutationIssue {
	return WorkbenchMutationIssue{
		Class:   workbenchMutationIssueClassSchema,
		Code:    workbenchResourceFieldCode(field, "UNEXPECTED"),
		Path:    path,
		Message: fmt.Sprintf("%s must be omitted when action is clear", field),
		Field:   field,
		Action:  action,
	}
}

func normalizeWorkbenchResourceClearFields(fields []string, action string) ([]string, []WorkbenchMutationIssue) {
	if len(fields) == 0 {
		return workbenchAllResourceFields(), nil
	}

	normalized := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	issues := []WorkbenchMutationIssue{}

	for idx, field := range fields {
		canonical, ok := workbenchCanonicalResourceField(field)
		if !ok {
			issues = append(issues, WorkbenchMutationIssue{
				Class:   workbenchMutationIssueClassSchema,
				Code:    "WB-RESOURCE-CLEAR-FIELD-INVALID",
				Path:    fmt.Sprintf("$.clearFields[%d]", idx),
				Message: fmt.Sprintf("unsupported clear field %q", field),
				Field:   strings.TrimSpace(field),
				Action:  action,
			})
			continue
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		normalized = append(normalized, canonical)
	}

	workbenchSortResourceFields(normalized)
	return normalized, issues
}

func workbenchCanonicalResourceField(field string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(field))
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")

	switch normalized {
	case "limitcpus":
		return workbenchResourceFieldLimitCPUs, true
	case "limitmemory":
		return workbenchResourceFieldLimitMemory, true
	case "reservationcpus":
		return workbenchResourceFieldReservationCPUs, true
	case "reservationmemory":
		return workbenchResourceFieldReservationMemory, true
	default:
		return "", false
	}
}

func workbenchAllResourceFields() []string {
	return []string{
		workbenchResourceFieldLimitCPUs,
		workbenchResourceFieldLimitMemory,
		workbenchResourceFieldReservationCPUs,
		workbenchResourceFieldReservationMemory,
	}
}

func workbenchSortResourceFields(fields []string) {
	order := map[string]int{
		workbenchResourceFieldLimitCPUs:         0,
		workbenchResourceFieldLimitMemory:       1,
		workbenchResourceFieldReservationCPUs:   2,
		workbenchResourceFieldReservationMemory: 3,
	}
	sort.SliceStable(fields, func(i, j int) bool {
		left, okLeft := order[fields[i]]
		right, okRight := order[fields[j]]
		if okLeft && okRight {
			return left < right
		}
		if okLeft != okRight {
			return okLeft
		}
		return fields[i] < fields[j]
	})
}

func workbenchResourceFieldCode(field, suffix string) string {
	codeField := strings.ToUpper(strings.TrimSpace(field))
	codeField = strings.ReplaceAll(codeField, "-", "_")
	codeField = strings.ReplaceAll(codeField, " ", "_")
	codeField = strings.ReplaceAll(codeField, "CPUS", "CPUS")
	return "WB-RESOURCE-" + codeField + "-" + strings.TrimSpace(strings.ToUpper(suffix))
}

func workbenchSnapshotHasService(services []WorkbenchComposeService, serviceName string) bool {
	target := strings.TrimSpace(serviceName)
	if target == "" {
		return false
	}
	for _, service := range services {
		if strings.EqualFold(strings.TrimSpace(service.ServiceName), target) {
			return true
		}
	}
	return false
}

func workbenchFindResourceIndex(resources []WorkbenchComposeResource, serviceName string) int {
	target := strings.TrimSpace(serviceName)
	if target == "" {
		return -1
	}
	for idx := range resources {
		if strings.EqualFold(strings.TrimSpace(resources[idx].ServiceName), target) {
			return idx
		}
	}
	return -1
}

func workbenchSetResourceField(resource *WorkbenchComposeResource, field string, value string) {
	trimmed := strings.TrimSpace(value)
	switch field {
	case workbenchResourceFieldLimitCPUs:
		resource.LimitCPUs = trimmed
	case workbenchResourceFieldLimitMemory:
		resource.LimitMemory = trimmed
	case workbenchResourceFieldReservationCPUs:
		resource.ReservationCPUs = trimmed
	case workbenchResourceFieldReservationMemory:
		resource.ReservationMemory = trimmed
	}
}

func workbenchResourceFieldValue(resource WorkbenchComposeResource, field string) string {
	switch field {
	case workbenchResourceFieldLimitCPUs:
		return strings.TrimSpace(resource.LimitCPUs)
	case workbenchResourceFieldLimitMemory:
		return strings.TrimSpace(resource.LimitMemory)
	case workbenchResourceFieldReservationCPUs:
		return strings.TrimSpace(resource.ReservationCPUs)
	case workbenchResourceFieldReservationMemory:
		return strings.TrimSpace(resource.ReservationMemory)
	default:
		return ""
	}
}

func workbenchIsEmptyResource(resource WorkbenchComposeResource) bool {
	return strings.TrimSpace(resource.LimitCPUs) == "" &&
		strings.TrimSpace(resource.LimitMemory) == "" &&
		strings.TrimSpace(resource.ReservationCPUs) == "" &&
		strings.TrimSpace(resource.ReservationMemory) == ""
}

func normalizeWorkbenchComposeResource(resource WorkbenchComposeResource) WorkbenchComposeResource {
	return WorkbenchComposeResource{
		ServiceName:       strings.TrimSpace(resource.ServiceName),
		LimitCPUs:         strings.TrimSpace(resource.LimitCPUs),
		LimitMemory:       strings.TrimSpace(resource.LimitMemory),
		ReservationCPUs:   strings.TrimSpace(resource.ReservationCPUs),
		ReservationMemory: strings.TrimSpace(resource.ReservationMemory),
	}
}

func cloneWorkbenchResource(resource WorkbenchComposeResource) *WorkbenchComposeResource {
	cloned := normalizeWorkbenchComposeResource(resource)
	return &cloned
}

func workbenchCountMatchingModules(modules []WorkbenchStackModule, target WorkbenchStackModule) int {
	count := 0
	for _, module := range modules {
		if workbenchModuleMatches(module, target) {
			count++
		}
	}
	return count
}

func workbenchModuleMatches(module WorkbenchStackModule, target WorkbenchStackModule) bool {
	return strings.EqualFold(strings.TrimSpace(module.ServiceName), strings.TrimSpace(target.ServiceName)) &&
		strings.EqualFold(strings.TrimSpace(module.ModuleType), strings.TrimSpace(target.ModuleType))
}

func workbenchIsSupportedModuleType(moduleType string) bool {
	_, ok := workbenchModuleDefaultPort(moduleType)
	return ok
}

func isWorkbenchCPUValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "$") {
		return true
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return false
	}
	return parsed > 0
}

func isWorkbenchMemoryValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "$") {
		return true
	}
	if !workbenchResourceMemoryPattern.MatchString(trimmed) {
		return false
	}

	numeric := strings.Builder{}
	for _, r := range trimmed {
		if (r >= '0' && r <= '9') || r == '.' {
			numeric.WriteRune(r)
			continue
		}
		break
	}
	if numeric.Len() == 0 {
		return false
	}

	parsed, err := strconv.ParseFloat(numeric.String(), 64)
	if err != nil {
		return false
	}
	return parsed > 0
}

func workbenchNormalizedStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func workbenchMutationIssueLess(left, right WorkbenchMutationIssue) bool {
	leftClass := strings.ToLower(strings.TrimSpace(left.Class))
	rightClass := strings.ToLower(strings.TrimSpace(right.Class))
	if leftClass != rightClass {
		return leftClass < rightClass
	}

	leftCode := strings.ToUpper(strings.TrimSpace(left.Code))
	rightCode := strings.ToUpper(strings.TrimSpace(right.Code))
	if leftCode != rightCode {
		return leftCode < rightCode
	}

	leftPath := strings.TrimSpace(left.Path)
	rightPath := strings.TrimSpace(right.Path)
	if leftPath != rightPath {
		return leftPath < rightPath
	}

	leftService := strings.ToLower(strings.TrimSpace(left.Service))
	rightService := strings.ToLower(strings.TrimSpace(right.Service))
	if leftService != rightService {
		return leftService < rightService
	}

	leftEntryKey := strings.ToLower(strings.TrimSpace(left.EntryKey))
	rightEntryKey := strings.ToLower(strings.TrimSpace(right.EntryKey))
	if leftEntryKey != rightEntryKey {
		return leftEntryKey < rightEntryKey
	}

	leftField := strings.ToLower(strings.TrimSpace(left.Field))
	rightField := strings.ToLower(strings.TrimSpace(right.Field))
	if leftField != rightField {
		return leftField < rightField
	}

	leftModule := strings.ToLower(strings.TrimSpace(left.ModuleType))
	rightModule := strings.ToLower(strings.TrimSpace(right.ModuleType))
	if leftModule != rightModule {
		return leftModule < rightModule
	}

	leftAction := strings.ToLower(strings.TrimSpace(left.Action))
	rightAction := strings.ToLower(strings.TrimSpace(right.Action))
	if leftAction != rightAction {
		return leftAction < rightAction
	}

	return strings.TrimSpace(left.Message) < strings.TrimSpace(right.Message)
}

func workbenchResourceMutationValidationError(
	snapshot WorkbenchStackSnapshot,
	summary WorkbenchResourceMutationSummary,
	issues []WorkbenchMutationIssue,
) error {
	normalizedIssues := append([]WorkbenchMutationIssue(nil), issues...)
	sort.SliceStable(normalizedIssues, func(i, j int) bool {
		return workbenchMutationIssueLess(normalizedIssues[i], normalizedIssues[j])
	})
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchValidationFailed, "invalid workbench resource mutation"),
		map[string]any{
			"project":           strings.TrimSpace(snapshot.ProjectName),
			"composePath":       strings.TrimSpace(snapshot.ComposePath),
			"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
			"revision":          snapshot.Revision,
			"action":            strings.TrimSpace(summary.Action),
			"selector":          summary.Selector,
			"issueCount":        len(normalizedIssues),
			"issues":            normalizedIssues,
			"summary":           summary,
		},
	)
}

func workbenchModuleMutationValidationError(
	snapshot WorkbenchStackSnapshot,
	summary WorkbenchModuleMutationSummary,
	issues []WorkbenchMutationIssue,
) error {
	normalizedIssues := append([]WorkbenchMutationIssue(nil), issues...)
	sort.SliceStable(normalizedIssues, func(i, j int) bool {
		return workbenchMutationIssueLess(normalizedIssues[i], normalizedIssues[j])
	})
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchValidationFailed, "invalid workbench module mutation"),
		map[string]any{
			"project":           strings.TrimSpace(snapshot.ProjectName),
			"composePath":       strings.TrimSpace(snapshot.ComposePath),
			"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
			"revision":          snapshot.Revision,
			"action":            strings.TrimSpace(summary.Action),
			"selector":          summary.Selector,
			"issueCount":        len(normalizedIssues),
			"issues":            normalizedIssues,
			"summary":           summary,
		},
	)
}

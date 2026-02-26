package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-notes/internal/config"
	"go-notes/internal/integrations/cloudflare"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

const ingressLogPrefix = "configuring tunnel ingress for "

type ProjectArchiveOptions struct {
	RemoveContainers bool `json:"removeContainers"`
	RemoveVolumes    bool `json:"removeVolumes"`
	RemoveIngress    bool `json:"removeIngress"`
	RemoveDNS        bool `json:"removeDns"`
}

type ProjectArchivePlan struct {
	Project          ProjectArchivePlanProject          `json:"project"`
	Defaults         ProjectArchiveOptions              `json:"defaults"`
	Hostnames        []string                           `json:"hostnames"`
	Containers       []ProjectArchivePlanContainer      `json:"containers"`
	ServiceExposures []ProjectArchivePlanServiceCleanup `json:"serviceExposures"`
	Ingress          []ProjectArchivePlanIngress        `json:"ingressRules"`
	DNSRecords       []ProjectArchivePlanDNSRecord      `json:"dnsRecords"`
	Warnings         []string                           `json:"warnings"`
}

type ProjectArchivePlanProject struct {
	Name           string `json:"name"`
	NormalizedName string `json:"normalizedName"`
	Path           string `json:"path"`
	Status         string `json:"status"`
}

type ProjectArchivePlanContainer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	Service string `json:"service"`
}

type ProjectArchivePlanServiceCleanup struct {
	JobID      uint   `json:"jobId"`
	Type       string `json:"type"`
	Hostname   string `json:"hostname"`
	Container  string `json:"container,omitempty"`
	Resolution string `json:"resolution"`
}

type ProjectArchivePlanIngress struct {
	Hostname string `json:"hostname"`
	Service  string `json:"service"`
	Source   string `json:"source"`
}

type ProjectArchivePlanDNSRecord struct {
	ID             string `json:"id"`
	ZoneID         string `json:"zoneId"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Content        string `json:"content"`
	Proxied        bool   `json:"proxied"`
	DeleteEligible bool   `json:"deleteEligible"`
	SkipReason     string `json:"skipReason,omitempty"`
}

type ProjectArchiveActor struct {
	UserID uint   `json:"userId"`
	Login  string `json:"login"`
}

type ProjectArchiveDNSDeleteTarget struct {
	ZoneID   string `json:"zoneId"`
	RecordID string `json:"recordId"`
	Hostname string `json:"hostname"`
	Content  string `json:"content"`
}

type ProjectArchiveTargets struct {
	Containers         []string                        `json:"containers"`
	Hostnames          []string                        `json:"hostnames"`
	ExposureContainers []string                        `json:"exposureContainers,omitempty"`
	ExposureHostnames  []string                        `json:"exposureHostnames,omitempty"`
	DNSRecords         []ProjectArchiveDNSDeleteTarget `json:"dnsRecords"`
}

type ProjectArchiveJobRequest struct {
	Project     string                `json:"project"`
	Options     ProjectArchiveOptions `json:"options"`
	Targets     ProjectArchiveTargets `json:"targets"`
	PlannedAt   time.Time             `json:"plannedAt"`
	RequestedBy ProjectArchiveActor   `json:"requestedBy"`
}

type ProjectArchiveService struct {
	cfg      config.Config
	projects repository.ProjectRepository
	settings *SettingsService
	jobs     *JobService
	host     *HostService
}

func NewProjectArchiveService(
	cfg config.Config,
	projects repository.ProjectRepository,
	settings *SettingsService,
	jobs *JobService,
	host *HostService,
) *ProjectArchiveService {
	return &ProjectArchiveService{
		cfg:      cfg,
		projects: projects,
		settings: settings,
		jobs:     jobs,
		host:     host,
	}
}

func DefaultProjectArchiveOptions() ProjectArchiveOptions {
	return ProjectArchiveOptions{
		RemoveContainers: true,
		RemoveVolumes:    false,
		RemoveIngress:    true,
		RemoveDNS:        true,
	}
}

func (s *ProjectArchiveService) Plan(ctx context.Context, projectName string) (ProjectArchivePlan, error) {
	runtimeCfg, err := s.resolveRuntimeConfig(ctx)
	if err != nil {
		return ProjectArchivePlan{}, err
	}

	resolved, err := resolveProjectPath(ctx, s.projects, runtimeCfg.TemplatesDir, projectName)
	if err != nil {
		return ProjectArchivePlan{}, err
	}

	plan := ProjectArchivePlan{
		Project: ProjectArchivePlanProject{
			Name:           resolved.RequestedName,
			NormalizedName: resolved.NormalizedName,
			Path:           resolved.ProjectDir,
			Status:         "unknown",
		},
		Defaults:         DefaultProjectArchiveOptions(),
		Hostnames:        []string{},
		Containers:       []ProjectArchivePlanContainer{},
		ServiceExposures: []ProjectArchivePlanServiceCleanup{},
		Ingress:          []ProjectArchivePlanIngress{},
		DNSRecords:       []ProjectArchivePlanDNSRecord{},
		Warnings:         []string{},
	}
	if resolved.ProjectRecord != nil && strings.TrimSpace(resolved.ProjectRecord.Status) != "" {
		plan.Project.Status = strings.TrimSpace(resolved.ProjectRecord.Status)
	}

	warnings := make(map[string]struct{})
	baseDomain := normalizeDomain(runtimeCfg.Domain)
	projectHostnames := s.discoverHostnames(ctx, resolved.NormalizedName, baseDomain, warnings)
	plan.ServiceExposures = s.planServiceExposures(ctx, resolved.NormalizedName, baseDomain, warnings)

	exposureHostnames, exposureContainers := archiveExposureTargets(plan.ServiceExposures)
	plan.Hostnames = mergeArchiveHostnames(projectHostnames, exposureHostnames)
	if len(plan.Hostnames) == 0 {
		addArchiveWarning(warnings, "no managed hostnames were discovered for this project")
	}
	plan.Containers = s.planContainers(ctx, resolved.NormalizedName, exposureContainers, warnings)

	cfClient := cloudflare.NewClient(runtimeCfg)
	plan.Ingress = s.planIngress(ctx, runtimeCfg, cfClient, plan.Hostnames, warnings)
	plan.DNSRecords = s.planDNSRecords(ctx, runtimeCfg, cfClient, plan.Hostnames, warnings)
	plan.Warnings = sortedArchiveWarnings(warnings)
	return plan, nil
}

func (s *ProjectArchiveService) Queue(
	ctx context.Context,
	projectName string,
	options ProjectArchiveOptions,
	actor ProjectArchiveActor,
) (*models.Job, ProjectArchivePlan, error) {
	if s.jobs == nil {
		return nil, ProjectArchivePlan{}, fmt.Errorf("job service unavailable")
	}

	plan, err := s.Plan(ctx, projectName)
	if err != nil {
		return nil, ProjectArchivePlan{}, err
	}

	options = normalizeArchiveOptions(options)
	targets := ProjectArchiveTargets{
		Containers:         []string{},
		Hostnames:          []string{},
		ExposureContainers: []string{},
		ExposureHostnames:  []string{},
		DNSRecords:         []ProjectArchiveDNSDeleteTarget{},
	}

	if options.RemoveContainers {
		for _, container := range plan.Containers {
			name := strings.TrimSpace(container.Name)
			if name == "" {
				continue
			}
			targets.Containers = append(targets.Containers, name)
		}
		for _, exposure := range plan.ServiceExposures {
			container := strings.TrimSpace(exposure.Container)
			if container == "" {
				continue
			}
			targets.ExposureContainers = append(targets.ExposureContainers, container)
		}
	}
	if options.RemoveIngress {
		targets.Hostnames = append(targets.Hostnames, plan.Hostnames...)
		for _, exposure := range plan.ServiceExposures {
			hostname := strings.ToLower(strings.TrimSpace(exposure.Hostname))
			if hostname == "" {
				continue
			}
			targets.ExposureHostnames = append(targets.ExposureHostnames, hostname)
		}
	}
	if options.RemoveDNS {
		for _, record := range plan.DNSRecords {
			if !record.DeleteEligible {
				continue
			}
			targets.DNSRecords = append(targets.DNSRecords, ProjectArchiveDNSDeleteTarget{
				ZoneID:   record.ZoneID,
				RecordID: record.ID,
				Hostname: record.Name,
				Content:  record.Content,
			})
		}
	}

	payload := ProjectArchiveJobRequest{
		Project:     plan.Project.NormalizedName,
		Options:     options,
		Targets:     targets,
		PlannedAt:   time.Now().UTC(),
		RequestedBy: actor,
	}

	job, err := s.jobs.Create(ctx, JobTypeProjectArchive, payload)
	if err != nil {
		return nil, ProjectArchivePlan{}, err
	}
	return job, plan, nil
}

func normalizeArchiveOptions(options ProjectArchiveOptions) ProjectArchiveOptions {
	normalized := options
	if !normalized.RemoveContainers {
		normalized.RemoveVolumes = false
	}
	return normalized
}

func (s *ProjectArchiveService) resolveRuntimeConfig(ctx context.Context) (config.Config, error) {
	if s.settings == nil {
		return s.cfg, nil
	}
	return s.settings.ResolveConfig(ctx)
}

func (s *ProjectArchiveService) planContainers(
	ctx context.Context,
	project string,
	exposureContainers []string,
	warnings map[string]struct{},
) []ProjectArchivePlanContainer {
	if s.host == nil {
		addArchiveWarning(warnings, "host service unavailable while planning container cleanup")
		return []ProjectArchivePlanContainer{}
	}

	containers, err := s.host.ListContainers(ctx, true)
	if err != nil {
		addArchiveWarning(warnings, fmt.Sprintf("failed to list project containers: %v", err))
		return []ProjectArchivePlanContainer{}
	}

	exposureSet := make(map[string]struct{}, len(exposureContainers))
	for _, value := range exposureContainers {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		exposureSet[trimmed] = struct{}{}
	}

	foundExposureContainers := make(map[string]struct{}, len(exposureSet))
	result := make([]ProjectArchivePlanContainer, 0)
	for _, container := range containers {
		containerName := strings.TrimSpace(container.Name)
		if containerName == "" {
			continue
		}

		_, exposureMatch := exposureSet[containerName]
		projectMatch := strings.EqualFold(strings.TrimSpace(container.Project), project)
		if !projectMatch && !exposureMatch {
			continue
		}

		service := strings.TrimSpace(container.Service)
		if exposureMatch {
			foundExposureContainers[containerName] = struct{}{}
			if service == "" {
				service = JobTypeQuickService
			}
		}

		result = append(result, ProjectArchivePlanContainer{
			ID:      container.ID,
			Name:    containerName,
			Image:   container.Image,
			Status:  container.Status,
			Service: service,
		})
	}

	for container := range exposureSet {
		if _, ok := foundExposureContainers[container]; ok {
			continue
		}
		addArchiveWarning(warnings, fmt.Sprintf("resolved quick_service container %s was not found on host while planning archive cleanup", container))
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (s *ProjectArchiveService) discoverHostnames(
	ctx context.Context,
	project string,
	baseDomain string,
	warnings map[string]struct{},
) []string {
	candidates := make(map[string]struct{})

	if s.jobs == nil {
		addArchiveWarning(warnings, "job service unavailable while discovering managed hostnames")
	} else {
		projectJobs, err := s.jobs.ListByProject(ctx, project)
		if err != nil {
			addArchiveWarning(warnings, fmt.Sprintf("failed to inspect project jobs: %v", err))
		} else {
			for _, job := range projectJobs {
				addHostnamesFromJobInput(candidates, job.Input, baseDomain)
				addHostnamesFromJobLogs(candidates, job.LogLines)
			}
		}
	}

	if baseDomain != "" {
		fallback := fmt.Sprintf("%s.%s", project, baseDomain)
		if ValidateDomain(fallback) == nil {
			candidates[fallback] = struct{}{}
		}
	}

	result := make([]string, 0, len(candidates))
	for hostname := range candidates {
		result = append(result, hostname)
	}
	sort.Strings(result)
	return result
}

func (s *ProjectArchiveService) planServiceExposures(
	ctx context.Context,
	project string,
	baseDomain string,
	warnings map[string]struct{},
) []ProjectArchivePlanServiceCleanup {
	if s.jobs == nil {
		addArchiveWarning(warnings, "job service unavailable while resolving service-exposure cleanup targets")
		return []ProjectArchivePlanServiceCleanup{}
	}

	jobs, err := s.jobs.List(ctx)
	if err != nil {
		addArchiveWarning(warnings, fmt.Sprintf("failed to inspect forward_local/quick_service jobs: %v", err))
		return []ProjectArchivePlanServiceCleanup{}
	}

	seen := make(map[string]struct{})
	result := make([]ProjectArchivePlanServiceCleanup, 0)
	for _, job := range jobs {
		switch strings.TrimSpace(job.Type) {
		case JobTypeForwardLocal:
			candidate, ok := resolveForwardLocalServiceExposure(project, baseDomain, job, warnings)
			if !ok {
				continue
			}
			key := fmt.Sprintf("%s:%s", candidate.Type, candidate.Hostname)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, candidate)
		case JobTypeQuickService:
			candidate, ok := resolveQuickServiceExposure(project, baseDomain, job, warnings)
			if !ok {
				continue
			}
			key := fmt.Sprintf("%s:%s:%s", candidate.Type, candidate.Hostname, candidate.Container)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, candidate)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Hostname == result[j].Hostname {
			if result[i].Type == result[j].Type {
				return result[i].JobID < result[j].JobID
			}
			return result[i].Type < result[j].Type
		}
		return result[i].Hostname < result[j].Hostname
	})
	return result
}

func resolveForwardLocalServiceExposure(
	project string,
	baseDomain string,
	job models.Job,
	warnings map[string]struct{},
) (ProjectArchivePlanServiceCleanup, bool) {
	var payload ForwardLocalRequest
	if err := json.Unmarshal([]byte(job.Input), &payload); err != nil {
		return ProjectArchivePlanServiceCleanup{}, false
	}

	project = strings.ToLower(strings.TrimSpace(project))
	subdomain := strings.ToLower(strings.TrimSpace(payload.Subdomain))

	resolution := ""
	switch {
	case subdomain == project:
		resolution = "subdomain.exact"
	case strings.HasPrefix(subdomain, project+"-"):
		resolution = "subdomain.prefix"
	default:
		if strings.HasPrefix(subdomain, project) {
			addArchiveWarning(
				warnings,
				fmt.Sprintf(
					"unresolved forward_local ownership for job %d (subdomain=%q): deterministic project mapping is unavailable",
					job.ID,
					subdomain,
				),
			)
		}
		return ProjectArchivePlanServiceCleanup{}, false
	}

	hostname, hostErr := resolveExposureHostname(subdomain, payload.Domain, baseDomain)
	if hostErr != "" {
		addArchiveWarning(
			warnings,
			fmt.Sprintf("unresolved forward_local ownership for job %d: %s", job.ID, hostErr),
		)
		return ProjectArchivePlanServiceCleanup{}, false
	}

	return ProjectArchivePlanServiceCleanup{
		JobID:      job.ID,
		Type:       JobTypeForwardLocal,
		Hostname:   hostname,
		Resolution: resolution,
	}, true
}

func resolveQuickServiceExposure(
	project string,
	baseDomain string,
	job models.Job,
	warnings map[string]struct{},
) (ProjectArchivePlanServiceCleanup, bool) {
	var payload QuickServiceRequest
	if err := json.Unmarshal([]byte(job.Input), &payload); err != nil {
		return ProjectArchivePlanServiceCleanup{}, false
	}

	project = strings.ToLower(strings.TrimSpace(project))
	subdomain := strings.ToLower(strings.TrimSpace(payload.Subdomain))

	resolution := ""
	switch {
	case subdomain == project:
		resolution = "subdomain.exact"
	case strings.HasPrefix(subdomain, project+"-"):
		resolution = "subdomain.prefix"
	default:
		if strings.HasPrefix(subdomain, project) {
			addArchiveWarning(
				warnings,
				fmt.Sprintf(
					"unresolved quick_service ownership for job %d (subdomain=%q): deterministic project mapping is unavailable",
					job.ID,
					subdomain,
				),
			)
		}
		return ProjectArchivePlanServiceCleanup{}, false
	}

	hostname, hostErr := resolveExposureHostname(subdomain, payload.Domain, baseDomain)
	if hostErr != "" {
		addArchiveWarning(
			warnings,
			fmt.Sprintf("unresolved quick_service ownership for job %d: %s", job.ID, hostErr),
		)
		return ProjectArchivePlanServiceCleanup{}, false
	}

	container := quickServiceContainerFromLogs(job.LogLines)
	if container == "" {
		addArchiveWarning(
			warnings,
			fmt.Sprintf(
				"quick_service job %d resolved to %s but container ownership is unresolved; container cleanup will be skipped for this exposure",
				job.ID,
				hostname,
			),
		)
	}

	return ProjectArchivePlanServiceCleanup{
		JobID:      job.ID,
		Type:       JobTypeQuickService,
		Hostname:   hostname,
		Container:  container,
		Resolution: resolution,
	}, true
}

func resolveExposureHostname(subdomain, domain, baseDomain string) (string, string) {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	if subdomain == "" {
		return "", "subdomain is missing"
	}

	domain = normalizeDomain(domain)
	if domain == "" {
		domain = normalizeDomain(baseDomain)
	}
	if domain == "" {
		return "", "domain is missing"
	}

	hostname := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%s.%s", subdomain, domain)))
	if ValidateDomain(hostname) != nil {
		return "", fmt.Sprintf("hostname %q is invalid", hostname)
	}
	return hostname, ""
}

func quickServiceContainerFromLogs(logLines string) string {
	const prefix = "starting docker container "
	for _, line := range strings.Split(logLines, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		container := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		if idx := strings.Index(container, " ("); idx != -1 {
			container = strings.TrimSpace(container[:idx])
		}
		if container == "" {
			continue
		}
		if err := validateContainerName(container); err != nil {
			continue
		}
		return container
	}
	return ""
}

func archiveExposureTargets(exposures []ProjectArchivePlanServiceCleanup) ([]string, []string) {
	hostnames := make([]string, 0, len(exposures))
	containers := make([]string, 0, len(exposures))
	for _, exposure := range exposures {
		hostname := strings.ToLower(strings.TrimSpace(exposure.Hostname))
		if hostname != "" {
			hostnames = append(hostnames, hostname)
		}
		container := strings.TrimSpace(exposure.Container)
		if container != "" {
			containers = append(containers, container)
		}
	}
	return dedupeHostnames(hostnames), dedupeStrings(containers)
}

func mergeArchiveHostnames(projectHostnames []string, exposureHostnames []string) []string {
	merged := make([]string, 0, len(projectHostnames)+len(exposureHostnames))
	merged = append(merged, projectHostnames...)
	merged = append(merged, exposureHostnames...)
	return dedupeHostnames(merged)
}

func addHostnamesFromJobInput(target map[string]struct{}, input string, baseDomain string) {
	if strings.TrimSpace(input) == "" {
		return
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(input), &payload); err != nil {
		return
	}

	addHostnameIfValid := func(value string) {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			return
		}
		if ValidateDomain(normalized) != nil {
			return
		}
		target[normalized] = struct{}{}
	}

	if rawHostname, ok := payload["hostname"].(string); ok {
		addHostnameIfValid(rawHostname)
	}

	subdomain, _ := payload["subdomain"].(string)
	domain, _ := payload["domain"].(string)
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	domain = normalizeDomain(domain)
	if domain == "" {
		domain = baseDomain
	}
	if subdomain != "" && domain != "" {
		addHostnameIfValid(fmt.Sprintf("%s.%s", subdomain, domain))
	}

	if rawTargets, ok := payload["targets"].(map[string]any); ok {
		if rawHostnames, ok := rawTargets["hostnames"].([]any); ok {
			for _, raw := range rawHostnames {
				value, ok := raw.(string)
				if !ok {
					continue
				}
				addHostnameIfValid(value)
			}
		}
	}
}

func addHostnamesFromJobLogs(target map[string]struct{}, logs string) {
	for _, line := range strings.Split(logs, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, ingressLogPrefix) {
			continue
		}
		hostname := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, ingressLogPrefix)))
		if hostname == "" {
			continue
		}
		if ValidateDomain(hostname) != nil {
			continue
		}
		target[hostname] = struct{}{}
	}
}

func (s *ProjectArchiveService) planIngress(
	ctx context.Context,
	runtimeCfg config.Config,
	cfClient *cloudflare.Client,
	hostnames []string,
	warnings map[string]struct{},
) []ProjectArchivePlanIngress {
	result := make([]ProjectArchivePlanIngress, 0)
	hostnameSet := make(map[string]struct{}, len(hostnames))
	for _, hostname := range hostnames {
		hostnameSet[strings.ToLower(strings.TrimSpace(hostname))] = struct{}{}
	}
	if len(hostnameSet) == 0 {
		return result
	}

	localRules, err := cloudflare.ListLocalIngressRules(runtimeCfg.CloudflaredConfig)
	if err != nil {
		addArchiveWarning(warnings, fmt.Sprintf("failed to inspect local ingress rules: %v", err))
	} else {
		for _, rule := range localRules {
			hostname := strings.ToLower(strings.TrimSpace(rule.Hostname))
			if _, ok := hostnameSet[hostname]; !ok {
				continue
			}
			result = append(result, ProjectArchivePlanIngress{
				Hostname: hostname,
				Service:  rule.Service,
				Source:   "local",
			})
		}
	}

	remoteRules, err := cfClient.ListIngressRules(ctx)
	if err != nil {
		addArchiveWarning(warnings, fmt.Sprintf("failed to inspect remote ingress rules: %v", err))
	} else {
		for _, rule := range remoteRules {
			hostname := strings.ToLower(strings.TrimSpace(rule.Hostname))
			if _, ok := hostnameSet[hostname]; !ok {
				continue
			}
			result = append(result, ProjectArchivePlanIngress{
				Hostname: hostname,
				Service:  rule.Service,
				Source:   "remote",
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Hostname == result[j].Hostname {
			return result[i].Source < result[j].Source
		}
		return result[i].Hostname < result[j].Hostname
	})
	if len(result) == 0 {
		addArchiveWarning(warnings, "no matching ingress rules were found for discovered hostnames")
	}
	return result
}

func (s *ProjectArchiveService) planDNSRecords(
	ctx context.Context,
	runtimeCfg config.Config,
	cfClient *cloudflare.Client,
	hostnames []string,
	warnings map[string]struct{},
) []ProjectArchivePlanDNSRecord {
	result := make([]ProjectArchivePlanDNSRecord, 0)
	if len(hostnames) == 0 {
		return result
	}

	zoneID := strings.TrimSpace(runtimeCfg.CloudflareZoneID)
	if zoneID == "" {
		addArchiveWarning(warnings, "cloudflare zone id is not configured; DNS cleanup preview is unavailable")
		return result
	}

	expectedTarget, err := cfClient.ExpectedTunnelCNAME(ctx)
	if err != nil {
		addArchiveWarning(warnings, fmt.Sprintf("failed to resolve tunnel DNS target: %v", err))
	}
	expectedTarget = strings.ToLower(strings.TrimSpace(expectedTarget))

	seen := make(map[string]struct{})
	for _, hostname := range hostnames {
		records, err := cfClient.ListDNSRecordsByName(ctx, hostname, zoneID)
		if err != nil {
			addArchiveWarning(warnings, fmt.Sprintf("failed to list DNS records for %s: %v", hostname, err))
			continue
		}
		for _, record := range records {
			key := strings.TrimSpace(record.ID)
			if key == "" {
				key = fmt.Sprintf("%s:%s:%s", zoneID, strings.ToLower(strings.TrimSpace(record.Name)), strings.ToLower(strings.TrimSpace(record.Content)))
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			planRecord := ProjectArchivePlanDNSRecord{
				ID:      strings.TrimSpace(record.ID),
				ZoneID:  zoneID,
				Name:    strings.ToLower(strings.TrimSpace(record.Name)),
				Type:    strings.ToUpper(strings.TrimSpace(record.Type)),
				Content: strings.TrimSpace(record.Content),
				Proxied: record.Proxied,
			}

			content := strings.ToLower(strings.TrimSpace(planRecord.Content))
			if planRecord.Type == "CNAME" && expectedTarget != "" && content == expectedTarget {
				planRecord.DeleteEligible = true
			} else {
				planRecord.DeleteEligible = false
				switch {
				case planRecord.Type != "CNAME":
					planRecord.SkipReason = "record type is not CNAME"
				case expectedTarget == "":
					planRecord.SkipReason = "expected tunnel target is unavailable"
				default:
					planRecord.SkipReason = fmt.Sprintf("record target %s does not match %s", planRecord.Content, expectedTarget)
				}
			}

			result = append(result, planRecord)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			if result[i].Type == result[j].Type {
				return result[i].ID < result[j].ID
			}
			return result[i].Type < result[j].Type
		}
		return result[i].Name < result[j].Name
	})

	if len(result) == 0 {
		addArchiveWarning(warnings, "no DNS records were found for discovered hostnames")
	}
	return result
}

func addArchiveWarning(target map[string]struct{}, warning string) {
	normalized := strings.TrimSpace(warning)
	if normalized == "" {
		return
	}
	target[normalized] = struct{}{}
}

func sortedArchiveWarnings(target map[string]struct{}) []string {
	result := make([]string, 0, len(target))
	for warning := range target {
		result = append(result, warning)
	}
	sort.Strings(result)
	return result
}

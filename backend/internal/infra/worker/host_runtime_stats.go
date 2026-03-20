package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go-notes/internal/infra/contract"
)

type hostRuntimeResource struct {
	TotalBytes     int64   `json:"totalBytes"`
	UsedBytes      int64   `json:"usedBytes"`
	FreeBytes      int64   `json:"freeBytes"`
	AvailableBytes int64   `json:"availableBytes,omitempty"`
	UsedPercent    float64 `json:"usedPercent"`
	SpeedMTs       int     `json:"speedMTs,omitempty"`
}

type hostRuntimeCPU struct {
	Model    string  `json:"model"`
	Cores    int     `json:"cores"`
	Threads  int     `json:"threads"`
	SpeedMHz float64 `json:"speedMHz,omitempty"`
}

type hostRuntimeGPU struct {
	Model    string  `json:"model"`
	SpeedMHz float64 `json:"speedMHz,omitempty"`
}

type hostRuntimeWorkloadUsage struct {
	Containers         int     `json:"containers"`
	RunningContainers  int     `json:"runningContainers"`
	CPUUsedPercent     float64 `json:"cpuUsedPercent"`
	MemoryUsedBytes    int64   `json:"memoryUsedBytes"`
	DiskUsedBytes      int64   `json:"diskUsedBytes"`
	MemorySharePercent float64 `json:"memorySharePercent"`
	DiskSharePercent   float64 `json:"diskSharePercent"`
}

type hostRuntimeStats struct {
	CollectedAt    string                              `json:"collectedAt"`
	Hostname       string                              `json:"hostname,omitempty"`
	UptimeSeconds  int64                               `json:"uptimeSeconds"`
	UptimeHuman    string                              `json:"uptimeHuman"`
	SystemImage    string                              `json:"systemImage"`
	Kernel         string                              `json:"kernel"`
	CPU            hostRuntimeCPU                      `json:"cpu"`
	GPU            *hostRuntimeGPU                     `json:"gpu,omitempty"`
	Memory         hostRuntimeResource                 `json:"memory"`
	Disk           hostRuntimeResource                 `json:"disk"`
	PanelUsage     hostRuntimeWorkloadUsage            `json:"panelUsage"`
	ProjectsUsage  hostRuntimeWorkloadUsage            `json:"projectsUsage"`
	ProjectsByName map[string]hostRuntimeWorkloadUsage `json:"projectsByName,omitempty"`
	Warnings       []string                            `json:"warnings,omitempty"`
}

type runtimeUsageCategory string

const (
	runtimeUsageUnknown  runtimeUsageCategory = "unknown"
	runtimeUsagePanel    runtimeUsageCategory = "panel"
	runtimeUsageProjects runtimeUsageCategory = "projects"
)

type runtimeUsageAccumulator struct {
	containers        int
	runningContainers int
	cpuUsedPercent    float64
	memoryUsedBytes   int64
	diskUsedBytes     int64
}

type runtimeUsageContainerMeta struct {
	category runtimeUsageCategory
	project  string
}

type dockerPSInventoryLine struct {
	ID     string `json:"ID"`
	Names  string `json:"Names"`
	Status string `json:"Status"`
	Labels string `json:"Labels"`
}

type dockerStatsLine struct {
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
}

type dockerContainerSizeLine struct {
	ID    string `json:"ID"`
	Names string `json:"Names"`
	Size  string `json:"Size"`
}

func (r *Runner) handleHostRuntimeStats(ctx context.Context, _ contract.Intent) taskOutcome {
	snapshot, warnings, err := collectHostRuntimeStats(ctx, r.exec, r.templatesDir)
	if err != nil {
		return taskOutcome{err: err, logTail: warnings}
	}
	data, err := structToMap(snapshot)
	if err != nil {
		return taskOutcome{
			err:     fmt.Errorf("encode host runtime stats payload: %w", err),
			logTail: warnings,
		}
	}
	return taskOutcome{
		logTail: warnings,
		data:    data,
	}
}

func collectHostRuntimeStats(ctx context.Context, exec commandExecutor, templatesDir string) (hostRuntimeStats, []string, error) {
	now := time.Now().UTC()
	stats := hostRuntimeStats{
		CollectedAt: now.Format(time.RFC3339),
		Hostname:    "Unknown host",
		CPU: hostRuntimeCPU{
			Model: "Unknown CPU",
			Cores: runtime.NumCPU(),
		},
		SystemImage: "Unknown system image",
		Warnings:    []string{},
	}
	stats.CPU.Threads = stats.CPU.Cores

	warnings := make([]string, 0)
	appendWarning := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}

	if uptimeSeconds, err := readHostUptimeSeconds(); err == nil {
		stats.UptimeSeconds = uptimeSeconds
		stats.UptimeHuman = formatUptime(uptimeSeconds)
	} else {
		appendWarning("uptime probe failed: %v", err)
	}

	if hostname := readHostName(ctx, exec); hostname != "" {
		stats.Hostname = hostname
	}

	if image, kernel, err := readSystemImageAndKernel(exec, ctx); err == nil {
		if image != "" {
			stats.SystemImage = image
		}
		stats.Kernel = kernel
	} else {
		appendWarning("system image probe failed: %v", err)
	}

	if model, cores, threads, speedMHz, err := readCPUInfo(); err == nil {
		if model != "" {
			stats.CPU.Model = model
		}
		if cores > 0 {
			stats.CPU.Cores = cores
		}
		if threads > 0 {
			stats.CPU.Threads = threads
		}
		if speedMHz > 0 {
			stats.CPU.SpeedMHz = speedMHz
		}
	} else {
		appendWarning("cpu probe failed: %v", err)
	}

	if stats.CPU.SpeedMHz <= 0 {
		if speedMHz, err := readCPUFrequencyMHzFromSysfs(); err == nil && speedMHz > 0 {
			stats.CPU.SpeedMHz = speedMHz
		}
	}

	if gpu, ok := detectGPUInfo(ctx, exec); ok {
		stats.GPU = gpu
	}

	if totalMemory, usedMemory, availableMemory, err := readMemoryUsageBytes(); err == nil {
		stats.Memory = buildResourceUsage(totalMemory, usedMemory, availableMemory)
		if speedMTs, ok := readMemorySpeedMTs(ctx, exec); ok {
			stats.Memory.SpeedMTs = speedMTs
		}
	} else {
		appendWarning("memory probe failed: %v", err)
	}

	if totalDisk, usedDisk, availableDisk, err := readRootDiskUsageBytes(ctx, exec, templatesDir); err == nil {
		stats.Disk = buildResourceUsage(totalDisk, usedDisk, availableDisk)
	} else {
		appendWarning("disk probe failed: %v", err)
	}

	panelUsage, projectsUsage, projectsByName, usageWarnings := readRuntimeUsageFromDocker(ctx, exec, templatesDir, stats.Memory.TotalBytes, stats.Disk.TotalBytes)
	warnings = append(warnings, usageWarnings...)
	stats.PanelUsage = panelUsage
	stats.ProjectsUsage = projectsUsage
	stats.ProjectsByName = projectsByName
	if len(warnings) > 0 {
		stats.Warnings = warnings
	}

	return stats, tailStrings(warnings, 25), nil
}

func structToMap(input any) (map[string]any, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func readHostUptimeSeconds() (int64, error) {
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0, fmt.Errorf("missing uptime fields")
	}
	secondsFloat, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	if secondsFloat < 0 {
		secondsFloat = 0
	}
	return int64(secondsFloat), nil
}

func readSystemImageAndKernel(exec commandExecutor, ctx context.Context) (string, string, error) {
	image := ""
	kernel := ""

	// Prefer docker daemon identity because it reflects the host runtime in containerized deployments.
	if exec != nil {
		if output, err := exec.Run(ctx, "", "docker", "info", "--format", "{{.OperatingSystem}}"); err == nil {
			image = strings.TrimSpace(string(output))
		}
		if output, err := exec.Run(ctx, "", "docker", "info", "--format", "{{.KernelVersion}}"); err == nil {
			kernel = strings.TrimSpace(string(output))
		}
	}

	if image == "" {
		if value, err := readOSReleasePrettyName("/etc/os-release"); err == nil {
			image = value
		}
	}

	if kernel == "" && exec != nil {
		if output, err := exec.Run(ctx, "", "uname", "-sr"); err == nil {
			kernel = strings.TrimSpace(string(output))
		}
	}
	if image == "" && kernel == "" {
		return "", "", fmt.Errorf("system identity unavailable")
	}
	return image, kernel, nil
}

func readHostName(ctx context.Context, exec commandExecutor) string {
	if exec != nil {
		if output, err := exec.Run(ctx, "", "docker", "info", "--format", "{{.Name}}"); err == nil {
			value := strings.TrimSpace(string(output))
			if value != "" {
				return value
			}
		}
	}
	if value, err := os.Hostname(); err == nil {
		return strings.TrimSpace(value)
	}
	return ""
}

func readOSReleasePrettyName(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "PRETTY_NAME=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "PRETTY_NAME="))
		value = strings.Trim(value, "\"'")
		if value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("pretty name unavailable")
}

func readCPUInfo() (string, int, int, float64, error) {
	raw, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", 0, 0, 0, err
	}
	model := ""
	logicalThreads := 0
	speedMHz := 0.0
	fallbackCores := 0
	coresBySocket := make(map[string]int)
	currentSocket := ""

	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			currentSocket = ""
			continue
		}
		key, value, ok := parseCPUInfoKeyValue(trimmed)
		if !ok {
			continue
		}
		switch key {
		case "processor":
			logicalThreads++
		case "model name":
			if model == "" {
				model = value
			}
		case "physical id":
			currentSocket = value
		case "cpu cores":
			if parsed, parseErr := strconv.Atoi(value); parseErr == nil && parsed > 0 {
				if fallbackCores <= 0 {
					fallbackCores = parsed
				}
				if currentSocket != "" {
					coresBySocket[currentSocket] = parsed
				}
			}
		case "cpu mhz":
			if speedMHz <= 0 {
				if parsed, parseErr := strconv.ParseFloat(value, 64); parseErr == nil && parsed > 0 {
					speedMHz = parsed
				}
			}
		}
	}

	if logicalThreads <= 0 {
		logicalThreads = runtime.NumCPU()
	}

	physicalCores := 0
	for _, cores := range coresBySocket {
		physicalCores += cores
	}
	if physicalCores <= 0 {
		physicalCores = fallbackCores
	}
	if physicalCores <= 0 {
		physicalCores = logicalThreads
	}

	if speedMHz <= 0 {
		if parsed, parseErr := readCPUFrequencyMHzFromSysfs(); parseErr == nil && parsed > 0 {
			speedMHz = parsed
		}
	}

	if model == "" && logicalThreads <= 0 {
		return "", 0, 0, 0, fmt.Errorf("cpu metadata unavailable")
	}
	return model, physicalCores, logicalThreads, speedMHz, nil
}

func parseCPUInfoKeyValue(raw string) (string, string, bool) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.ToLower(strings.TrimSpace(parts[0]))
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

func readCPUFrequencyMHzFromSysfs() (float64, error) {
	paths := []string{
		"/sys/devices/system/cpu/cpu0/cpufreq/scaling_max_freq",
		"/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq",
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
		if err != nil || value <= 0 {
			continue
		}
		if value > 100000 {
			return mathRound(value/1000, 2), nil
		}
		return mathRound(value, 2), nil
	}
	return 0, fmt.Errorf("cpu speed unavailable")
}

func detectGPUInfo(ctx context.Context, exec commandExecutor) (*hostRuntimeGPU, bool) {
	if exec == nil {
		return nil, false
	}
	if output, err := exec.Run(ctx, "", "nvidia-smi", "--query-gpu=name,clocks.current.graphics", "--format=csv,noheader,nounits"); err == nil {
		lines := parseOutputLines(output)
		if len(lines) > 0 {
			model, speedMHz := parseNvidiaGPUInfoLine(lines[0])
			if model != "" {
				return &hostRuntimeGPU{Model: model, SpeedMHz: speedMHz}, true
			}
		}
	}
	if output, err := exec.Run(ctx, "", "nvidia-smi", "--query-gpu=name", "--format=csv,noheader"); err == nil {
		lines := parseOutputLines(output)
		if len(lines) > 0 {
			model := strings.TrimSpace(lines[0])
			if model != "" {
				return &hostRuntimeGPU{Model: model}, true
			}
		}
	}
	output, err := exec.Run(ctx, "", "lspci")
	if err != nil {
		return nil, false
	}
	re := regexp.MustCompile(`(?i)(vga compatible controller|3d controller|display controller):\s*(.+)$`)
	for _, line := range parseOutputLines(output) {
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 3 {
			model := strings.TrimSpace(matches[2])
			if model != "" {
				return &hostRuntimeGPU{Model: model}, true
			}
		}
	}
	return nil, false
}

func parseNvidiaGPUInfoLine(raw string) (string, float64) {
	parts := strings.Split(raw, ",")
	model := ""
	if len(parts) > 0 {
		model = strings.TrimSpace(parts[0])
	}
	speedMHz := 0.0
	if len(parts) > 1 {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil && parsed > 0 {
			speedMHz = mathRound(parsed, 2)
		}
	}
	return model, speedMHz
}

func readMemoryUsageBytes() (int64, int64, int64, error) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, err
	}
	values := map[string]int64{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		values[key] = value * 1024
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if total <= 0 {
		return 0, 0, 0, fmt.Errorf("memory total unavailable")
	}
	if available < 0 {
		available = 0
	}
	used := total - available
	if used < 0 {
		used = 0
	}
	return total, used, available, nil
}

var memorySpeedRegex = regexp.MustCompile(`(?i)(configured memory speed|speed):\s*([0-9]+)\s*(MT/s|MHz)`)

func readMemorySpeedMTs(ctx context.Context, exec commandExecutor) (int, bool) {
	if exec == nil {
		return 0, false
	}
	output, err := exec.Run(ctx, "", "dmidecode", "-t", "memory")
	if err != nil {
		return 0, false
	}
	speedMTs := 0
	for _, line := range parseOutputLines(output) {
		matches := memorySpeedRegex.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		parsed, parseErr := strconv.Atoi(matches[2])
		if parseErr != nil || parsed <= 0 {
			continue
		}
		if parsed > speedMTs {
			speedMTs = parsed
		}
	}
	if speedMTs <= 0 {
		return 0, false
	}
	return speedMTs, true
}

func readRootDiskUsageBytes(ctx context.Context, exec commandExecutor, templatesDir string) (int64, int64, int64, error) {
	if exec == nil {
		return 0, 0, 0, fmt.Errorf("executor unavailable")
	}
	probePath := resolveDiskProbePath(templatesDir)
	output, err := exec.Run(ctx, "", "df", "-B1", probePath)
	if err != nil && probePath != "/" {
		output, err = exec.Run(ctx, "", "df", "-B1", "/")
	}
	if err != nil {
		return 0, 0, 0, err
	}
	lines := parseOutputLines(output)
	if len(lines) < 2 {
		return 0, 0, 0, fmt.Errorf("unexpected df output")
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, 0, 0, fmt.Errorf("unexpected df row format")
	}
	total, err := strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	used, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	available := int64(0)
	if len(fields) >= 4 {
		if parsed, parseErr := strconv.ParseInt(strings.TrimSpace(fields[3]), 10, 64); parseErr == nil && parsed >= 0 {
			available = parsed
		}
	}
	return total, used, available, nil
}

func resolveDiskProbePath(templatesDir string) string {
	candidates := []string{
		strings.TrimSpace(templatesDir),
		"/templates",
		"/home",
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		return candidate
	}
	return "/"
}

func readRuntimeUsageFromDocker(
	ctx context.Context,
	exec commandExecutor,
	templatesDir string,
	totalMemoryBytes int64,
	totalDiskBytes int64,
) (hostRuntimeWorkloadUsage, hostRuntimeWorkloadUsage, map[string]hostRuntimeWorkloadUsage, []string) {
	panelAccum := runtimeUsageAccumulator{}
	projectAccum := runtimeUsageAccumulator{}
	projectAccumByName := make(map[string]*runtimeUsageAccumulator)
	warnings := make([]string, 0)

	if exec == nil {
		warnings = append(warnings, "docker usage probe skipped: executor unavailable")
		return finalizeRuntimeUsage(panelAccum, totalMemoryBytes, totalDiskBytes), finalizeRuntimeUsage(projectAccum, totalMemoryBytes, totalDiskBytes), finalizeRuntimeUsageByProject(projectAccumByName, totalMemoryBytes, totalDiskBytes), warnings
	}

	localProjectNames := listLocalProjectNames(templatesDir)
	inventoryByName, inventoryByID, countsPanel, countsProjects, countsByProject, err := readDockerInventory(ctx, exec, localProjectNames)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("docker inventory probe failed: %v", err))
	} else {
		panelAccum.containers = countsPanel.containers
		panelAccum.runningContainers = countsPanel.runningContainers
		projectAccum.containers = countsProjects.containers
		projectAccum.runningContainers = countsProjects.runningContainers
		projectAccumByName = countsByProject
	}

	if err := readDockerMemoryUsage(ctx, exec, inventoryByName, &panelAccum, &projectAccum, projectAccumByName); err != nil {
		warnings = append(warnings, fmt.Sprintf("docker memory probe failed: %v", err))
	}
	if err := readDockerDiskUsage(ctx, exec, inventoryByName, inventoryByID, &panelAccum, &projectAccum, projectAccumByName); err != nil {
		warnings = append(warnings, fmt.Sprintf("docker disk probe failed: %v", err))
	}

	return finalizeRuntimeUsage(panelAccum, totalMemoryBytes, totalDiskBytes), finalizeRuntimeUsage(projectAccum, totalMemoryBytes, totalDiskBytes), finalizeRuntimeUsageByProject(projectAccumByName, totalMemoryBytes, totalDiskBytes), warnings
}

func readDockerInventory(
	ctx context.Context,
	exec commandExecutor,
	localProjectNames map[string]struct{},
) (map[string]runtimeUsageContainerMeta, map[string]runtimeUsageContainerMeta, runtimeUsageAccumulator, runtimeUsageAccumulator, map[string]*runtimeUsageAccumulator, error) {
	output, err := exec.Run(ctx, "", "docker", "ps", "-a", "--format", "{{json .}}")
	if err != nil {
		return nil, nil, runtimeUsageAccumulator{}, runtimeUsageAccumulator{}, nil, err
	}

	inventoryByName := make(map[string]runtimeUsageContainerMeta)
	inventoryByID := make(map[string]runtimeUsageContainerMeta)
	projectAccums := make(map[string]*runtimeUsageAccumulator)
	panel := runtimeUsageAccumulator{}
	projects := runtimeUsageAccumulator{}

	for _, line := range parseOutputLines(output) {
		var row dockerPSInventoryLine
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		labels := parseDockerLabelString(row.Labels)
		composeProject := strings.ToLower(strings.TrimSpace(labels["com.docker.compose.project"]))
		containerName := strings.ToLower(strings.TrimSpace(row.Names))
		category := classifyRuntimeUsageContainer(containerName, composeProject, localProjectNames)
		if category == runtimeUsageUnknown {
			continue
		}
		meta := runtimeUsageContainerMeta{category: category}
		if category == runtimeUsageProjects {
			meta.project = composeProject
		}
		if containerName != "" {
			inventoryByName[containerName] = meta
		}
		containerID := strings.TrimSpace(row.ID)
		if containerID != "" {
			inventoryByID[containerID] = meta
		}
		switch category {
		case runtimeUsagePanel:
			panel.containers++
			if isRunningContainerStatus(row.Status) {
				panel.runningContainers++
			}
		case runtimeUsageProjects:
			projects.containers++
			if isRunningContainerStatus(row.Status) {
				projects.runningContainers++
			}
			project := ensureProjectAccumulator(projectAccums, composeProject)
			if project != nil {
				project.containers++
				if isRunningContainerStatus(row.Status) {
					project.runningContainers++
				}
			}
		}
	}

	return inventoryByName, inventoryByID, panel, projects, projectAccums, nil
}

func readDockerMemoryUsage(
	ctx context.Context,
	exec commandExecutor,
	inventoryByName map[string]runtimeUsageContainerMeta,
	panel *runtimeUsageAccumulator,
	projects *runtimeUsageAccumulator,
	projectAccums map[string]*runtimeUsageAccumulator,
) error {
	output, err := exec.Run(ctx, "", "docker", "stats", "--no-stream", "--format", "{{json .}}")
	if err != nil {
		return err
	}
	for _, line := range parseOutputLines(output) {
		var row dockerStatsLine
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(row.Name))
		meta, ok := inventoryByName[name]
		if !ok || meta.category == runtimeUsageUnknown {
			continue
		}

		cpuPercent, hasCPU := parsePercentToFloat(row.CPUPerc)
		usedPart := strings.TrimSpace(strings.SplitN(row.MemUsage, "/", 2)[0])
		usedBytes, hasMemory := parseHumanSizeToBytes(usedPart)
		if !hasMemory || usedBytes < 0 {
			usedBytes = 0
			hasMemory = false
		}

		switch meta.category {
		case runtimeUsagePanel:
			if hasCPU {
				panel.cpuUsedPercent += cpuPercent
			}
			if hasMemory {
				panel.memoryUsedBytes += usedBytes
			}
		case runtimeUsageProjects:
			if hasCPU {
				projects.cpuUsedPercent += cpuPercent
			}
			if hasMemory {
				projects.memoryUsedBytes += usedBytes
			}
			project := ensureProjectAccumulator(projectAccums, meta.project)
			if project != nil {
				if hasCPU {
					project.cpuUsedPercent += cpuPercent
				}
				if hasMemory {
					project.memoryUsedBytes += usedBytes
				}
			}
		}
	}
	return nil
}

func readDockerDiskUsage(
	ctx context.Context,
	exec commandExecutor,
	inventoryByName map[string]runtimeUsageContainerMeta,
	inventoryByID map[string]runtimeUsageContainerMeta,
	panel *runtimeUsageAccumulator,
	projects *runtimeUsageAccumulator,
	projectAccums map[string]*runtimeUsageAccumulator,
) error {
	output, err := exec.Run(ctx, "", "docker", "ps", "-as", "--format", "{{json .}}")
	if err != nil {
		return err
	}
	for _, line := range parseOutputLines(output) {
		var row dockerContainerSizeLine
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		meta := runtimeUsageContainerMeta{category: runtimeUsageUnknown}
		name := strings.ToLower(strings.TrimSpace(row.Names))
		if name != "" {
			meta = inventoryByName[name]
		}
		if meta.category == runtimeUsageUnknown {
			id := strings.TrimSpace(row.ID)
			if id != "" {
				meta = inventoryByID[id]
			}
		}
		if meta.category == runtimeUsageUnknown {
			continue
		}
		sizePart := strings.TrimSpace(strings.SplitN(row.Size, "(", 2)[0])
		sizeBytes, ok := parseHumanSizeToBytes(sizePart)
		if !ok || sizeBytes < 0 {
			continue
		}
		if meta.category == runtimeUsagePanel {
			panel.diskUsedBytes += sizeBytes
		} else if meta.category == runtimeUsageProjects {
			projects.diskUsedBytes += sizeBytes
			project := ensureProjectAccumulator(projectAccums, meta.project)
			if project != nil {
				project.diskUsedBytes += sizeBytes
			}
		}
	}
	return nil
}

func classifyRuntimeUsageContainer(containerName, composeProject string, localProjectNames map[string]struct{}) runtimeUsageCategory {
	if composeProject == "warp-panel" || strings.HasPrefix(containerName, "warp-panel-") {
		return runtimeUsagePanel
	}
	if composeProject == "" {
		return runtimeUsageUnknown
	}
	if _, ok := localProjectNames[composeProject]; ok {
		return runtimeUsageProjects
	}
	if composeProject != "warp-panel" {
		return runtimeUsageProjects
	}
	return runtimeUsageUnknown
}

func listLocalProjectNames(templatesDir string) map[string]struct{} {
	names := make(map[string]struct{})
	root := strings.TrimSpace(templatesDir)
	if root == "" {
		return names
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return names
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(entry.Name()))
		if name != "" {
			names[name] = struct{}{}
		}
	}
	return names
}

func parseDockerLabelString(raw string) map[string]string {
	labels := make(map[string]string)
	for _, token := range strings.Split(raw, ",") {
		pair := strings.SplitN(strings.TrimSpace(token), "=", 2)
		if len(pair) != 2 {
			continue
		}
		key := strings.TrimSpace(pair[0])
		value := strings.TrimSpace(pair[1])
		if key != "" {
			labels[key] = value
		}
	}
	return labels
}

func isRunningContainerStatus(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	return strings.HasPrefix(normalized, "up") || strings.Contains(normalized, "running")
}

func parseOutputLines(output []byte) []string {
	rawLines := strings.Split(string(output), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

var humanSizePattern = regexp.MustCompile(`^\s*([0-9]+(?:\.[0-9]+)?)\s*([A-Za-z]+)?\s*$`)

func parseHumanSizeToBytes(raw string) (int64, bool) {
	matches := humanSizePattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(matches) < 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, false
	}
	unit := strings.ToUpper(strings.TrimSpace(matches[2]))
	if unit == "" {
		unit = "B"
	}
	multiplier := float64(1)
	switch unit {
	case "B":
		multiplier = 1
	case "K", "KB", "KIB":
		multiplier = 1024
	case "M", "MB", "MIB":
		multiplier = 1024 * 1024
	case "G", "GB", "GIB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB", "TIB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, false
	}
	return int64(value * multiplier), true
}

func buildResourceUsage(total, used, available int64) hostRuntimeResource {
	if total < 0 {
		total = 0
	}
	if used < 0 {
		used = 0
	}
	if total > 0 && used > total {
		used = total
	}
	free := total - used
	if free < 0 {
		free = 0
	}
	if available < 0 || available > total {
		available = 0
	}
	if available == 0 {
		available = free
	}
	return hostRuntimeResource{
		TotalBytes:     total,
		UsedBytes:      used,
		FreeBytes:      free,
		AvailableBytes: available,
		UsedPercent:    percentOf(used, total),
	}
}

func finalizeRuntimeUsage(accum runtimeUsageAccumulator, totalMemoryBytes, totalDiskBytes int64) hostRuntimeWorkloadUsage {
	return hostRuntimeWorkloadUsage{
		Containers:         accum.containers,
		RunningContainers:  accum.runningContainers,
		CPUUsedPercent:     clampRuntimePercent(accum.cpuUsedPercent),
		MemoryUsedBytes:    accum.memoryUsedBytes,
		DiskUsedBytes:      accum.diskUsedBytes,
		MemorySharePercent: percentOf(accum.memoryUsedBytes, totalMemoryBytes),
		DiskSharePercent:   percentOf(accum.diskUsedBytes, totalDiskBytes),
	}
}

func finalizeRuntimeUsageByProject(
	projectAccums map[string]*runtimeUsageAccumulator,
	totalMemoryBytes int64,
	totalDiskBytes int64,
) map[string]hostRuntimeWorkloadUsage {
	if len(projectAccums) == 0 {
		return nil
	}
	usage := make(map[string]hostRuntimeWorkloadUsage, len(projectAccums))
	for projectName, projectAccum := range projectAccums {
		if projectAccum == nil {
			continue
		}
		usage[projectName] = finalizeRuntimeUsage(*projectAccum, totalMemoryBytes, totalDiskBytes)
	}
	if len(usage) == 0 {
		return nil
	}
	return usage
}

func ensureProjectAccumulator(
	projectAccums map[string]*runtimeUsageAccumulator,
	projectName string,
) *runtimeUsageAccumulator {
	key := strings.ToLower(strings.TrimSpace(projectName))
	if key == "" {
		return nil
	}
	existing, ok := projectAccums[key]
	if ok && existing != nil {
		return existing
	}
	created := &runtimeUsageAccumulator{}
	projectAccums[key] = created
	return created
}

func parsePercentToFloat(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimSuffix(trimmed, "%")
	if trimmed == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false
	}
	if value < 0 {
		value = 0
	}
	return value, true
}

func clampRuntimePercent(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value > 100 {
		value = 100
	}
	return mathRound(value, 2)
}

func percentOf(value, total int64) float64 {
	if total <= 0 || value <= 0 {
		return 0
	}
	percent := (float64(value) / float64(total)) * 100
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return mathRound(percent, 2)
}

func mathRound(value float64, precision int) float64 {
	if precision <= 0 {
		return float64(int64(value + 0.5))
	}
	pow := 1.0
	for i := 0; i < precision; i++ {
		pow *= 10
	}
	return float64(int64(value*pow+0.5)) / pow
}

func formatUptime(seconds int64) string {
	if seconds <= 0 {
		return "0m"
	}
	days := seconds / 86400
	seconds %= 86400
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60

	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	return strings.Join(parts, " ")
}

func tailStrings(lines []string, limit int) []string {
	if limit <= 0 || len(lines) == 0 {
		return nil
	}
	if len(lines) <= limit {
		out := make([]string, len(lines))
		copy(out, lines)
		return out
	}
	start := len(lines) - limit
	out := make([]string, len(lines[start:]))
	copy(out, lines[start:])
	return out
}

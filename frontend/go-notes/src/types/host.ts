export interface DockerPortBinding {
  hostIp: string
  hostPort: number
  containerPort: number
  proto: string
  published: boolean
}

export interface DockerContainer {
  id: string
  name: string
  image: string
  status: string
  ports: string
  createdAt: string
  runningFor: string
  service: string
  project: string
  portBindings: DockerPortBinding[]
}

export interface DockerReadDiagnostic {
  scope: string
  status: string
  code: string
  message: string
  sourceCode?: string
  taskType?: string
}

export interface DockerUsageEntry {
  count: number
  size: string
}

export interface DockerUsageProjectCounts {
  containers: number
  images: number
  volumes: number
}

export interface DockerUsageSummary {
  totalSize: string
  images: DockerUsageEntry
  containers: DockerUsageEntry
  volumes: DockerUsageEntry
  buildCache?: DockerUsageEntry
  project?: string
  projectCounts?: DockerUsageProjectCounts
}

export interface HostRuntimeCPU {
  model: string
  cores: number
  threads: number
  speedMHz?: number
}

export interface HostRuntimeGPU {
  model: string
  speedMHz?: number
}

export interface HostRuntimeResource {
  totalBytes: number
  usedBytes: number
  freeBytes: number
  availableBytes?: number
  usedPercent: number
}

export interface HostRuntimeMemorySnapshot {
  totalBytes: number
  freeBytes: number
  availableBytes?: number
  speedMTs?: number
}

export interface HostRuntimeWorkloadSnapshot {
  containers: number
  runningContainers: number
  diskUsedBytes: number
  diskSharePercent: number
}

export interface HostRuntimeSnapshot {
  collectedAt: string
  hostname?: string
  uptimeSeconds: number
  uptimeHuman: string
  systemImage: string
  kernel: string
  cpu: HostRuntimeCPU
  gpu?: HostRuntimeGPU
  memory: HostRuntimeMemorySnapshot
  disk: HostRuntimeResource
  panel: HostRuntimeWorkloadSnapshot
  projects: HostRuntimeWorkloadSnapshot
  projectsByName?: Record<string, HostRuntimeWorkloadSnapshot>
  warnings?: string[]
}

export interface HostRuntimeHostStreamUsage {
  memoryUsedBytes: number
  memoryUsedPercent: number
  memoryFreeBytes: number
  memoryAvailableBytes?: number
}

export interface HostRuntimeWorkloadStreamUsage {
  cpuUsedPercent: number
  memoryUsedBytes: number
  memorySharePercent: number
}

export interface HostRuntimeStreamSample {
  collectedAt: string
  mode: string
  intervalMs: number
  host: HostRuntimeHostStreamUsage
  panel: HostRuntimeWorkloadStreamUsage
  projects: HostRuntimeWorkloadStreamUsage
  projectsByName?: Record<string, HostRuntimeWorkloadStreamUsage>
  warnings?: string[]
}

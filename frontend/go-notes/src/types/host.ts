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

export interface HostRuntimeResource {
  totalBytes: number
  usedBytes: number
  freeBytes: number
  availableBytes?: number
  usedPercent: number
  speedMTs?: number
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

export interface HostRuntimeWorkloadUsage {
  containers: number
  runningContainers: number
  cpuUsedPercent: number
  memoryUsedBytes: number
  diskUsedBytes: number
  memorySharePercent: number
  diskSharePercent: number
}

export interface HostRuntimeStats {
  collectedAt: string
  hostname?: string
  uptimeSeconds: number
  uptimeHuman: string
  systemImage: string
  kernel: string
  cpu: HostRuntimeCPU
  gpu?: HostRuntimeGPU
  memory: HostRuntimeResource
  disk: HostRuntimeResource
  panelUsage: HostRuntimeWorkloadUsage
  projectsUsage: HostRuntimeWorkloadUsage
  projectsByName?: Record<string, HostRuntimeWorkloadUsage>
  warnings?: string[]
}

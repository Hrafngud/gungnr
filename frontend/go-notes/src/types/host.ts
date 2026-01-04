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

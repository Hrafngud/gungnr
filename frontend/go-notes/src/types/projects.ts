export interface Project {
  id: number
  name: string
  repoUrl: string
  path: string
  proxyPort: number
  dbPort: number
  status: string
  createdAt: string
  updatedAt: string
}

export interface LocalProject {
  name: string
  path: string
}

export interface ProjectDetailProject {
  name: string
  normalizedName: string
  record?: Project
}

export interface ProjectDetailRuntime {
  path: string
  source: string
  composeFiles: string[]
  envPath: string
  envExists: boolean
}

export interface ProjectPublishedPort {
  container: string
  service: string
  hostIp: string
  hostPort: number
  containerPort: number
  proto: string
}

export interface ProjectDetailNetwork {
  proxyPort: number
  dbPort: number
  publishedPorts: ProjectPublishedPort[]
}

export interface ProjectContainer {
  id: string
  name: string
  image: string
  status: string
  ports: string
  createdAt: string
  runningFor: string
  service: string
  project: string
  portBindings: Array<{
    hostIp: string
    hostPort: number
    containerPort: number
    proto: string
    published: boolean
  }>
}

export interface ProjectDetail {
  project: ProjectDetailProject
  runtime: ProjectDetailRuntime
  network: ProjectDetailNetwork
  containers: ProjectContainer[]
}

export interface ProjectEnvRead {
  path: string
  exists: boolean
  sizeBytes: number
  updatedAt?: string
  content: string
}

export interface ProjectEnvWrite {
  path: string
  sizeBytes: number
  updatedAt: string
  backupPath?: string
}

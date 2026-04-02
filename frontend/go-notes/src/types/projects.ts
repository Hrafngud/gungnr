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

export interface ProjectDetailDiagnostic {
  scope: string
  status: string
  code: string
  message: string
  sourceCode?: string
  taskType?: string
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
  diagnostics?: ProjectDetailDiagnostic[]
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

export interface ProjectArchiveOptions {
  removeContainers: boolean
  removeVolumes: boolean
  removeIngress: boolean
  removeDns: boolean
}

export interface ProjectArchivePlanProject {
  name: string
  normalizedName: string
  path: string
  status: string
}

export interface ProjectArchivePlanContainer {
  id: string
  name: string
  image: string
  status: string
  service: string
}

export interface ProjectArchivePlanServiceExposure {
  jobId: number
  type: string
  hostname: string
  container?: string
  resolution: string
}

export interface ProjectArchivePlanIngressRule {
  hostname: string
  service: string
  source: 'local' | 'remote' | string
}

export interface ProjectArchivePlanDNSRecord {
  id: string
  zoneId: string
  name: string
  type: string
  content: string
  proxied: boolean
  deleteEligible: boolean
  skipReason?: string
}

export interface ProjectArchivePlan {
  project: ProjectArchivePlanProject
  defaults: ProjectArchiveOptions
  hostnames: string[]
  containers: ProjectArchivePlanContainer[]
  serviceExposures: ProjectArchivePlanServiceExposure[]
  ingressRules: ProjectArchivePlanIngressRule[]
  dnsRecords: ProjectArchivePlanDNSRecord[]
  warnings: string[]
}

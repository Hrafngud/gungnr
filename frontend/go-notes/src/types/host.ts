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

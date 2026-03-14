import axios from 'axios'
import mockData from '@/mock.json'
import type {
  WorkbenchDependencyGraph,
  WorkbenchDependencyNodeStatus,
  WorkbenchPortSelector,
  WorkbenchStackPort,
  WorkbenchStackSnapshot,
} from '@/types/workbench'

export class ApiError extends Error {
  code?: string
  docsUrl?: string
  fields?: Record<string, string>
  details?: unknown

  constructor(
    message: string,
    options?: {
      code?: string
      docsUrl?: string
      fields?: Record<string, string>
      details?: unknown
    },
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = options?.code
    this.docsUrl = options?.docsUrl
    this.fields = options?.fields
    this.details = options?.details
  }
}

const defaultBaseUrl = (() => {
  if (typeof window === 'undefined') {
    return 'http://localhost:8080'
  }

  const hostname = window.location.hostname
  const localHosts = new Set(['localhost', '127.0.0.1'])
  if (localHosts.has(hostname)) {
    return 'http://localhost:8080'
  }

  return window.location.origin
})()

const envBaseUrl = import.meta.env.VITE_API_BASE_URL
const resolvedBaseUrl = normalizeBaseUrl(resolveBaseUrl(envBaseUrl, defaultBaseUrl))
const apiBaseUrl = normalizeBrowserBaseUrl(resolvedBaseUrl).replace(/\/$/, '')

function resolveBaseUrl(envUrl: string | undefined, fallback: string): string {
  if (!envUrl) return fallback

  if (typeof window === 'undefined') {
    return envUrl
  }

  if (isAbsoluteLocalhostUrl(envUrl) && !isLocalHost(window.location.hostname)) {
    return window.location.origin
  }

  return envUrl
}

function normalizeBaseUrl(value: string): string {
  if (typeof window === 'undefined') {
    return value
  }

  if (!value.startsWith('http://') && !value.startsWith('https://')) {
    return value
  }

  try {
    const parsed = new URL(value)
    if (window.location.protocol === 'https:' && parsed.protocol === 'http:') {
      parsed.protocol = 'https:'
      return parsed.toString()
    }
  } catch {
    return value
  }

  return value
}

function normalizeBrowserBaseUrl(value: string): string {
  if (typeof window === 'undefined') {
    return value
  }

  if (window.location.protocol !== 'https:') {
    return value
  }

  if (!value || value === '/') {
    return window.location.origin
  }

  if (value.startsWith('http://')) {
    try {
      const parsed = new URL(value)
      parsed.protocol = 'https:'
      return parsed.toString()
    } catch {
      return window.location.origin
    }
  }

  return value
}

function isAbsoluteLocalhostUrl(value: string): boolean {
  if (!value.startsWith('http://') && !value.startsWith('https://')) {
    return false
  }

  try {
    const parsed = new URL(value)
    return isLocalHost(parsed.hostname)
  } catch {
    return false
  }
}

function isLocalHost(hostname: string): boolean {
  const normalized = hostname.toLowerCase()
  return normalized === 'localhost' || normalized === '127.0.0.1' || normalized === '::1'
}

export const api = axios.create({
  baseURL: apiBaseUrl,
  headers: {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  },
  withCredentials: true,
  timeout: 15000,
})

const mockFlagKey = 'gungnr:mock'
const workbenchSnapshotPathPattern = /^\/api\/v1\/projects\/([^/]+)\/workbench$/
const workbenchGraphPathPattern = /^\/api\/v1\/projects\/([^/]+)\/workbench\/graph$/
const workbenchComposePreviewPathPattern = /^\/api\/v1\/projects\/([^/]+)\/workbench\/compose\/preview$/
const workbenchPortMutatePathPattern = /^\/api\/v1\/projects\/([^/]+)\/workbench\/ports\/mutate$/
const workbenchPortSuggestPathPattern = /^\/api\/v1\/projects\/([^/]+)\/workbench\/ports\/suggest$/

interface MockWorkbenchState {
  initial: WorkbenchStackSnapshot
  current: WorkbenchStackSnapshot
}

const mockWorkbenchStateByProject = new Map<string, MockWorkbenchState>()

function isMockEnabled(): boolean {
  if (typeof window === 'undefined') return false
  try {
    return window.localStorage.getItem(mockFlagKey) === '1'
  } catch {
    return false
  }
}

function getMockKey(method: string, url?: string | null): string | null {
  if (!url) return null
  return `${method.toUpperCase()} ${url}`
}

function cloneMockValue<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function normalizeMockPath(url?: string | null): string | null {
  if (!url) return null
  const [pathname] = url.split('?')
  return pathname || null
}

function decodeProjectSegment(segment: string): string {
  try {
    return decodeURIComponent(segment)
  } catch {
    return segment
  }
}

function parseMockPayload(data: unknown): Record<string, unknown> {
  if (!data) return {}
  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data)
      if (parsed && typeof parsed === 'object') {
        return parsed as Record<string, unknown>
      }
    } catch {
      return {}
    }
    return {}
  }
  if (typeof data === 'object') {
    return data as Record<string, unknown>
  }
  return {}
}

function mockSnapshotFromStaticData(projectName: string): WorkbenchStackSnapshot | null {
  const keys = [
    `GET /api/v1/projects/${encodeURIComponent(projectName)}/workbench`,
    `GET /api/v1/projects/${projectName}/workbench`,
  ]

  for (const key of keys) {
    const response = (mockData as Record<string, unknown>)[key]
    if (!response || typeof response !== 'object') continue
    const stack = (response as { stack?: unknown }).stack
    if (!stack || typeof stack !== 'object') continue
    return cloneMockValue(stack as WorkbenchStackSnapshot)
  }

  return null
}

function getMockWorkbenchState(projectName: string): MockWorkbenchState | null {
  const cached = mockWorkbenchStateByProject.get(projectName)
  if (cached) return cached

  const initialSnapshot = mockSnapshotFromStaticData(projectName)
  if (!initialSnapshot) return null

  const state: MockWorkbenchState = {
    initial: cloneMockValue(initialSnapshot),
    current: cloneMockValue(initialSnapshot),
  }
  mockWorkbenchStateByProject.set(projectName, state)
  return state
}

function normalizeSelectorProtocol(selector: WorkbenchPortSelector): string {
  return selector.protocol?.trim().toLowerCase() || 'tcp'
}

function normalizeSelectorHostIp(selector: WorkbenchPortSelector): string {
  return selector.hostIp?.trim() || '0.0.0.0'
}

function normalizePortProtocol(port: WorkbenchStackPort): string {
  return port.protocol?.trim().toLowerCase() || 'tcp'
}

function normalizePortHostIp(port: WorkbenchStackPort): string {
  return port.hostIp?.trim() || '0.0.0.0'
}

function parseMockSelector(payload: Record<string, unknown>): WorkbenchPortSelector | null {
  const selectorValue = payload.selector
  if (!selectorValue || typeof selectorValue !== 'object') return null
  const selectorRecord = selectorValue as Record<string, unknown>

  const serviceName = stringOrUndefined(selectorRecord.serviceName)
  const containerPort = numberOrUndefined(selectorRecord.containerPort)
  if (!serviceName || containerPort == null || !Number.isInteger(containerPort)) return null

  const protocol = stringOrUndefined(selectorRecord.protocol)
  const hostIp = stringOrUndefined(selectorRecord.hostIp)
  return {
    serviceName,
    containerPort,
    ...(protocol ? { protocol } : {}),
    ...(hostIp ? { hostIp } : {}),
  }
}

function yamlQuoted(value: string): string {
  return JSON.stringify(value)
}

function uniqueList(values: string[]): string[] {
  const seen = new Set<string>()
  const ordered: string[] = []
  values.forEach((value) => {
    const normalized = value.trim()
    if (!normalized || seen.has(normalized)) return
    seen.add(normalized)
    ordered.push(normalized)
  })
  return ordered
}

function mockMountPath(serviceName: string, volumeName: string): string {
  const safeService = serviceName.replace(/[^a-zA-Z0-9_.-]+/g, '-')
  const safeVolume = volumeName.replace(/[^a-zA-Z0-9_.-]+/g, '-')
  return `/var/lib/${safeService}/${safeVolume}`
}

function mockComposePortBinding(port: WorkbenchStackPort): string | null {
  const protocol = normalizePortProtocol(port)
  const hostIp = normalizePortHostIp(port)
  const containerPort = String(port.containerPort)

  const hostPortRaw = port.hostPortRaw?.trim()
  if (hostPortRaw) {
    const hasExplicitPair = hostPortRaw.includes(':')
    const mappingBase = hasExplicitPair ? hostPortRaw : `${hostPortRaw}:${containerPort}`
    const mappingWithHostIp =
      hostIp !== '0.0.0.0' && !hasExplicitPair ? `${hostIp}:${mappingBase}` : mappingBase
    return protocol === 'tcp' ? mappingWithHostIp : `${mappingWithHostIp}/${protocol}`
  }

  if (typeof port.hostPort === 'number' && Number.isInteger(port.hostPort)) {
    const mappingBase = hostIp !== '0.0.0.0' ? `${hostIp}:${port.hostPort}:${containerPort}` : `${port.hostPort}:${containerPort}`
    return protocol === 'tcp' ? mappingBase : `${mappingBase}/${protocol}`
  }

  return null
}

function buildMockComposePreview(stack: WorkbenchStackSnapshot): string {
  const dependenciesByService = new Map<string, string[]>()
  stack.dependencies.forEach((dependency) => {
    const serviceName = dependency.serviceName.trim()
    const dependsOn = dependency.dependsOn.trim()
    if (!serviceName || !dependsOn) return
    const current = dependenciesByService.get(serviceName) ?? []
    current.push(dependsOn)
    dependenciesByService.set(serviceName, current)
  })

  const portsByService = new Map<string, WorkbenchStackPort[]>()
  stack.ports.forEach((port) => {
    const serviceName = port.serviceName.trim()
    if (!serviceName) return
    const current = portsByService.get(serviceName) ?? []
    current.push(port)
    portsByService.set(serviceName, current)
  })

  const resourceByService = new Map<string, WorkbenchStackSnapshot['resources'][number]>()
  stack.resources.forEach((resource) => {
    const serviceName = resource.serviceName.trim()
    if (!serviceName) return
    resourceByService.set(serviceName, resource)
  })

  const envByService = new Map<string, Array<{ variable: string; expression: string }>>()
  stack.envRefs.forEach((ref) => {
    const serviceName = ref.serviceName?.trim() || ''
    const variable = ref.variable?.trim() || ''
    const expression = ref.expression?.trim() || ''
    if (!serviceName || !variable || !expression) return
    const current = envByService.get(serviceName) ?? []
    current.push({ variable, expression })
    envByService.set(serviceName, current)
  })

  const networksByService = new Map<string, string[]>()
  stack.networkRefs.forEach((networkRef) => {
    const serviceName = networkRef.serviceName.trim()
    const networkName = networkRef.networkName.trim()
    if (!serviceName || !networkName) return
    const current = networksByService.get(serviceName) ?? []
    current.push(networkName)
    networksByService.set(serviceName, current)
  })

  const volumesByService = new Map<string, string[]>()
  stack.volumeRefs.forEach((volumeRef) => {
    const serviceName = volumeRef.serviceName.trim()
    const volumeName = volumeRef.volumeName.trim()
    if (!serviceName || !volumeName) return
    const current = volumesByService.get(serviceName) ?? []
    current.push(volumeName)
    volumesByService.set(serviceName, current)
  })

  const allNetworks = uniqueList(stack.networkRefs.map((ref) => ref.networkName))
  const allVolumes = uniqueList(stack.volumeRefs.map((ref) => ref.volumeName))
  const lines: string[] = [
    '# Mock-generated compose preview from current Workbench snapshot.',
    'version: "3.9"',
  ]

  if (stack.services.length === 0) {
    lines.push('services: {}')
    return lines.join('\n')
  }

  lines.push('services:')
  stack.services.forEach((service, serviceIndex) => {
    const serviceName = service.serviceName.trim()
    if (!serviceName) return

    if (serviceIndex > 0) {
      lines.push('')
    }
    lines.push(`  ${serviceName}:`)

    const image = service.image?.trim()
    if (image) {
      lines.push(`    image: ${yamlQuoted(image)}`)
    }

    const buildSource = service.buildSource?.trim()
    if (buildSource) {
      lines.push(`    build: ${yamlQuoted(buildSource)}`)
    }

    const restartPolicy = service.restartPolicy?.trim()
    if (restartPolicy) {
      lines.push(`    restart: ${yamlQuoted(restartPolicy)}`)
    }

    const dependencies = uniqueList(dependenciesByService.get(serviceName) ?? [])
    if (dependencies.length > 0) {
      lines.push('    depends_on:')
      dependencies.forEach((dependsOn) => {
        lines.push(`      - ${dependsOn}`)
      })
    }

    const bindings = (portsByService.get(serviceName) ?? [])
      .map((port) => mockComposePortBinding(port))
      .filter((binding): binding is string => Boolean(binding))
    if (bindings.length > 0) {
      lines.push('    ports:')
      bindings.forEach((binding) => {
        lines.push(`      - ${yamlQuoted(binding)}`)
      })
    }

    const environmentEntries = envByService.get(serviceName) ?? []
    if (environmentEntries.length > 0) {
      lines.push('    environment:')
      environmentEntries.forEach((entry) => {
        lines.push(`      ${entry.variable}: ${yamlQuoted(entry.expression)}`)
      })
    }

    const resource = resourceByService.get(serviceName)
    if (resource) {
      const limitCpus = resource.limitCpus?.trim() || ''
      const limitMemory = resource.limitMemory?.trim() || ''
      const reservationCpus = resource.reservationCpus?.trim() || ''
      const reservationMemory = resource.reservationMemory?.trim() || ''
      const hasLimits = Boolean(limitCpus || limitMemory)
      const hasReservations = Boolean(reservationCpus || reservationMemory)

      if (hasLimits || hasReservations) {
        lines.push('    deploy:')
        lines.push('      resources:')
        if (hasLimits) {
          lines.push('        limits:')
          if (limitCpus) {
            lines.push(`          cpus: ${yamlQuoted(limitCpus)}`)
          }
          if (limitMemory) {
            lines.push(`          memory: ${yamlQuoted(limitMemory)}`)
          }
        }
        if (hasReservations) {
          lines.push('        reservations:')
          if (reservationCpus) {
            lines.push(`          cpus: ${yamlQuoted(reservationCpus)}`)
          }
          if (reservationMemory) {
            lines.push(`          memory: ${yamlQuoted(reservationMemory)}`)
          }
        }
      }
    }

    const networks = uniqueList(networksByService.get(serviceName) ?? [])
    if (networks.length > 0) {
      lines.push('    networks:')
      networks.forEach((networkName) => {
        lines.push(`      - ${networkName}`)
      })
    }

    const volumes = uniqueList(volumesByService.get(serviceName) ?? [])
    if (volumes.length > 0) {
      lines.push('    volumes:')
      volumes.forEach((volumeName) => {
        lines.push(`      - ${volumeName}:${mockMountPath(serviceName, volumeName)}`)
      })
    }
  })

  if (allNetworks.length > 0) {
    lines.push('')
    lines.push('networks:')
    allNetworks.forEach((networkName) => {
      lines.push(`  ${networkName}: {}`)
    })
  }

  if (allVolumes.length > 0) {
    lines.push('')
    lines.push('volumes:')
    allVolumes.forEach((volumeName) => {
      lines.push(`  ${volumeName}: {}`)
    })
  }

  return lines.join('\n')
}

function resolveMockWorkbenchComposePreview(
  projectName: string,
): { preview: Record<string, unknown> } | null {
  const state = getMockWorkbenchState(projectName)
  if (!state) return null

  const stack = state.current
  return {
    preview: {
      compose: buildMockComposePreview(stack),
      metadata: {
        revision: stack.revision,
        sourceFingerprint: stack.sourceFingerprint,
      },
    },
  }
}

function resolveMockWorkbenchGraph(projectName: string): { graph: WorkbenchDependencyGraph } | null {
  const state = getMockWorkbenchState(projectName)
  if (!state) return null

  const stack = state.current
  const serviceNames = new Set<string>()
  stack.services.forEach((service) => {
    const serviceName = service.serviceName.trim()
    if (serviceName) serviceNames.add(serviceName)
  })
  stack.dependencies.forEach((dependency) => {
    const toService = dependency.serviceName.trim()
    const fromService = dependency.dependsOn.trim()
    if (toService) serviceNames.add(toService)
    if (fromService) serviceNames.add(fromService)
  })

  const sortedNames = [...serviceNames].sort((a, b) => a.localeCompare(b))
  const statusByService = new Map<string, WorkbenchDependencyNodeStatus>()

  const nodes = sortedNames.map((serviceName) => {
    const status: WorkbenchDependencyNodeStatus = 'running'
    statusByService.set(serviceName, status)
    return {
      serviceName,
      status,
      statusText: 'mock runtime: running',
      containerCount: 1,
      runningCount: 1,
      healthyCount: 1,
      failedCount: 0,
    }
  })

  const seenEdges = new Set<string>()
  const edges = stack.dependencies
    .map((dependency) => {
      const toService = dependency.serviceName.trim()
      const fromService = dependency.dependsOn.trim()
      if (!toService || !fromService) return null

      const edgeKey = `${fromService.toLowerCase()}->${toService.toLowerCase()}`
      if (seenEdges.has(edgeKey)) return null
      seenEdges.add(edgeKey)

      return {
        key: `${fromService}->${toService}`,
        fromService,
        toService,
        sourceStatus: statusByService.get(fromService) ?? 'unknown',
        failureSource: false,
      }
    })
    .filter((edge): edge is WorkbenchDependencyGraph['edges'][number] => Boolean(edge))
    .sort((a, b) => {
      const fromDiff = a.fromService.localeCompare(b.fromService)
      if (fromDiff !== 0) return fromDiff
      return a.toService.localeCompare(b.toService)
    })

  return {
    graph: {
      projectName: stack.projectName,
      revision: stack.revision,
      sourceFingerprint: stack.sourceFingerprint,
      nodes,
      edges,
      warnings: ['mock runtime data is synthetic'],
    },
  }
}

function findMockPortIndex(stack: WorkbenchStackSnapshot, selector: WorkbenchPortSelector): number {
  return stack.ports.findIndex(
    (port) =>
      port.serviceName.trim() === selector.serviceName.trim() &&
      port.containerPort === selector.containerPort &&
      normalizePortProtocol(port) === normalizeSelectorProtocol(selector) &&
      normalizePortHostIp(port) === normalizeSelectorHostIp(selector),
  )
}

function portStateSignature(port: WorkbenchStackPort): string {
  return [
    port.assignmentStrategy || '',
    port.allocationStatus || '',
    port.hostPort != null ? String(port.hostPort) : '',
    port.hostPortRaw || '',
  ].join('|')
}

function bumpMockWorkbenchRevision(stack: WorkbenchStackSnapshot) {
  const revision = Number.isInteger(stack.revision) ? stack.revision : 0
  const nextRevision = Math.max(1, revision + 1)
  stack.revision = nextRevision
  stack.sourceFingerprint = `sha256:mock-workbench-rev${nextRevision}`
}

function clampHostPort(value: number): number {
  const truncated = Math.trunc(value)
  if (truncated < 1) return 1
  if (truncated > 65535) return 65535
  return truncated
}

function collectOccupiedHostPorts(
  stack: WorkbenchStackSnapshot,
  protocol: string,
  hostIp: string,
  excludedIndex: number,
): Set<number> {
  const occupied = new Set<number>()
  stack.ports.forEach((port, index) => {
    if (index === excludedIndex) return
    if (normalizePortProtocol(port) !== protocol) return
    if (normalizePortHostIp(port) !== hostIp) return
    if (typeof port.hostPort === 'number' && Number.isInteger(port.hostPort)) {
      occupied.add(port.hostPort)
    }
  })
  return occupied
}

function findNextAvailableHostPort(
  stack: WorkbenchStackSnapshot,
  desiredHostPort: number,
  selector: WorkbenchPortSelector,
  excludedIndex: number,
): number | undefined {
  const protocol = normalizeSelectorProtocol(selector)
  const hostIp = normalizeSelectorHostIp(selector)
  const occupied = collectOccupiedHostPorts(stack, protocol, hostIp, excludedIndex)
  let candidate = clampHostPort(desiredHostPort)
  while (candidate <= 65535) {
    if (!occupied.has(candidate)) {
      return candidate
    }
    candidate += 1
  }
  return undefined
}

function initialPortForSelector(
  state: MockWorkbenchState,
  selector: WorkbenchPortSelector,
): WorkbenchStackPort | null {
  const index = findMockPortIndex(state.initial, selector)
  if (index < 0) return null
  return state.initial.ports[index] || null
}

function resolveMockWorkbenchPortMutation(
  projectName: string,
  payload: Record<string, unknown>,
): { stack: WorkbenchStackSnapshot; mutation: Record<string, unknown> } | null {
  const state = getMockWorkbenchState(projectName)
  if (!state) return null

  const action = stringOrUndefined(payload.action)
  if (action !== 'set_manual' && action !== 'clear_manual') return null
  const selector = parseMockSelector(payload)
  if (!selector) return null

  const stack = state.current
  const portIndex = findMockPortIndex(stack, selector)
  if (portIndex < 0) {
    return {
      stack: cloneMockValue(stack),
      mutation: {
        changed: false,
        action,
        selector,
        source: 'mock',
        status: 'unavailable',
        message: 'No matching Workbench port row was found in mock mode.',
      },
    }
  }

  const targetPort = stack.ports[portIndex]
  if (!targetPort) {
    return {
      stack: cloneMockValue(stack),
      mutation: {
        changed: false,
        action,
        selector,
        source: 'mock',
        status: 'unavailable',
        message: 'No matching Workbench port row was found in mock mode.',
      },
    }
  }
  const before = portStateSignature(targetPort)
  const previousStrategy = targetPort.assignmentStrategy?.trim().toLowerCase() || 'auto'
  const previousHostPort =
    typeof targetPort.hostPort === 'number' && Number.isInteger(targetPort.hostPort)
      ? targetPort.hostPort
      : undefined

  let status: string | undefined
  let message = ''
  let assignedHostPort: number | undefined
  let requestedHostPort: number | undefined
  let preferredHostPort: number | undefined

  if (action === 'set_manual') {
    const manualHostPort = numberOrUndefined(payload.manualHostPort)
    if (manualHostPort == null || !Number.isInteger(manualHostPort)) {
      return {
        stack: cloneMockValue(stack),
        mutation: {
          changed: false,
          action,
          selector,
          source: 'mock',
          status: 'unavailable',
          message: 'Mock mode requires an integer manual host port.',
        },
      }
    }

    requestedHostPort = clampHostPort(manualHostPort)
    preferredHostPort = requestedHostPort
    assignedHostPort = requestedHostPort

    const protocol = normalizeSelectorProtocol(selector)
    const hostIp = normalizeSelectorHostIp(selector)
    const occupied = collectOccupiedHostPorts(stack, protocol, hostIp, portIndex)
    const hasConflict = occupied.has(requestedHostPort)

    targetPort.assignmentStrategy = 'manual'
    targetPort.hostPort = requestedHostPort
    delete targetPort.hostPortRaw
    targetPort.allocationStatus = hasConflict ? 'conflict' : 'assigned'
    status = targetPort.allocationStatus
    message = hasConflict
      ? 'Mock mode flagged this host port as conflicting with another mapping.'
      : 'Mock mode saved the manual host-port assignment.'
  } else {
    const initialPort = initialPortForSelector(state, selector)
    targetPort.assignmentStrategy = 'auto'

    if (
      initialPort &&
      typeof initialPort.hostPort !== 'number' &&
      typeof initialPort.hostPortRaw === 'string' &&
      initialPort.hostPortRaw.trim()
    ) {
      targetPort.hostPortRaw = initialPort.hostPortRaw
      delete targetPort.hostPort
      targetPort.allocationStatus = 'unresolved'
      status = 'unresolved'
      message = 'Mock mode restored the unresolved compose host-port expression.'
    } else {
      preferredHostPort =
        (initialPort &&
        typeof initialPort.hostPort === 'number' &&
        Number.isInteger(initialPort.hostPort)
          ? initialPort.hostPort
          : undefined) ??
        previousHostPort ??
        selector.containerPort
      const availableHostPort = findNextAvailableHostPort(
        stack,
        preferredHostPort,
        selector,
        portIndex,
      )
      if (availableHostPort == null) {
        delete targetPort.hostPort
        delete targetPort.hostPortRaw
        targetPort.allocationStatus = 'unavailable'
        status = 'unavailable'
        message = 'Mock mode could not find an available host port while returning to auto allocation.'
      } else {
        targetPort.hostPort = availableHostPort
        delete targetPort.hostPortRaw
        targetPort.allocationStatus = 'assigned'
        assignedHostPort = availableHostPort
        status = 'assigned'
        message = 'Mock mode restored auto allocation for this host-port mapping.'
      }
    }
  }

  const after = portStateSignature(targetPort)
  const changed = before !== after
  if (changed) {
    bumpMockWorkbenchRevision(stack)
  }

  return {
    stack: cloneMockValue(stack),
    mutation: {
      changed,
      action,
      selector,
      source: 'mock',
      status,
      message,
      previousStrategy,
      currentStrategy: targetPort.assignmentStrategy?.trim().toLowerCase() || 'auto',
      previousHostPort,
      requestedHostPort,
      preferredHostPort,
      assignedHostPort,
      attempts: 1,
    },
  }
}

function resolveMockWorkbenchPortSuggestions(
  projectName: string,
  payload: Record<string, unknown>,
): { stack: WorkbenchStackSnapshot; suggestions: Record<string, unknown> } | null {
  const state = getMockWorkbenchState(projectName)
  if (!state) return null

  const selector = parseMockSelector(payload)
  if (!selector) return null

  const stack = state.current
  const portIndex = findMockPortIndex(stack, selector)
  if (portIndex < 0) {
    return {
      stack: cloneMockValue(stack),
      suggestions: {
        selector,
        source: 'mock',
        limit: 5,
        suggestionCount: 0,
        suggestions: [],
      },
    }
  }

  const targetPort = stack.ports[portIndex]
  if (!targetPort) {
    return {
      stack: cloneMockValue(stack),
      suggestions: {
        selector,
        source: 'mock',
        limit: 5,
        suggestionCount: 0,
        suggestions: [],
      },
    }
  }
  const rawLimit = numberOrUndefined(payload.limit)
  const limit = rawLimit && Number.isInteger(rawLimit) ? clampHostPort(rawLimit) : 5
  const boundedLimit = Math.min(Math.max(limit, 1), 20)
  const currentHostPort =
    typeof targetPort.hostPort === 'number' && Number.isInteger(targetPort.hostPort)
      ? targetPort.hostPort
      : undefined
  const initialPort = initialPortForSelector(state, selector)
  const preferredHostPort =
    currentHostPort ??
    (initialPort &&
    typeof initialPort.hostPort === 'number' &&
    Number.isInteger(initialPort.hostPort)
      ? initialPort.hostPort
      : undefined) ??
    selector.containerPort

  const startCandidate = currentHostPort != null ? currentHostPort + 1 : preferredHostPort
  const suggestions: Array<{ hostPort: number; rank: number }> = []
  let candidate = clampHostPort(startCandidate)
  while (candidate <= 65535 && suggestions.length < boundedLimit) {
    const available = findNextAvailableHostPort(stack, candidate, selector, portIndex)
    if (available == null) break
    suggestions.push({
      hostPort: available,
      rank: suggestions.length + 1,
    })
    candidate = available + 1
  }

  return {
    stack: cloneMockValue(stack),
    suggestions: {
      selector,
      source: 'mock',
      preferredHostPort,
      currentHostPort,
      currentStrategy: targetPort.assignmentStrategy?.trim().toLowerCase() || 'auto',
      currentStatus: targetPort.allocationStatus?.trim().toLowerCase() || 'unavailable',
      limit: boundedLimit,
      suggestionCount: suggestions.length,
      suggestions,
    },
  }
}

function objectOrNull(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function isNonNull<T>(value: T | null | undefined): value is T {
  return value !== null && value !== undefined
}

function normalizeMockNetbirdModeLabel(mode: string): string {
  const normalized = mode.trim().toLowerCase()
  if (normalized === 'mode_a') return 'Mode A'
  if (normalized === 'mode_b') return 'Mode B'
  return 'Legacy'
}

function normalizeMockNetbirdDefaultActionLabel(action: string): string {
  const normalized = action.trim().toLowerCase()
  if (normalized.includes('deny')) return 'Deny by default'
  if (normalized.includes('allow') || normalized.includes('accept')) return 'Allow by default'
  return action.trim() || 'Unknown'
}

function normalizeMockNetbirdDefaultActionTone(action: string): string {
  const normalized = action.trim().toLowerCase()
  if (normalized.includes('deny') || normalized.includes('block')) return 'ok'
  if (normalized.includes('allow') || normalized.includes('accept')) return 'warn'
  return 'neutral'
}

function normalizeMockNetbirdGraphResponse(value: unknown): unknown {
  const response = objectOrNull(value)
  if (!response) return value
  const rawGraph = objectOrNull(response.graph)
  if (!rawGraph) return value

  const normalizedCurrentMode = stringOrUndefined(rawGraph.currentMode) ?? 'legacy'
  const normalizedConfiguredMode = stringOrUndefined(rawGraph.configuredMode) ?? normalizedCurrentMode
  const defaultAction = stringOrUndefined(rawGraph.defaultAction) ?? 'deny-by-default'

  interface NormalizedMockNetbirdNode {
    id: string
    label: string
    kind: string
    kindLabel: string
    tone: string
    groupName?: string
    projectName?: string
    projectId?: number
  }

  interface NormalizedMockNetbirdEdge {
    id: string
    from: string
    to: string
    fromLabel: string
    toLabel: string
    policy: string
    rule: string
    ruleLabel: string
    action: string
    protocol: string
    ports: string[]
    bidirectional: boolean
    tone: string
  }

  const rawNodes = Array.isArray(rawGraph.nodes) ? rawGraph.nodes : []
  const nodes = rawNodes
    .map((rawNode, index) => {
      const node = objectOrNull(rawNode)
      if (!node) return null
      const id = stringOrUndefined(node.id) ?? `node-${index + 1}`
      const label = stringOrUndefined(node.label) ?? id
      const kind = stringOrUndefined(node.kind) ?? 'node'
      const groupName = stringOrUndefined(node.groupName)
      const projectName = stringOrUndefined(node.projectName)
      const projectID = numberOrUndefined(node.projectId)

      let kindLabel = stringOrUndefined(node.kindLabel) ?? ''
      let tone = stringOrUndefined(node.tone) ?? ''
      const normalizedKind = kind.trim().toLowerCase()
      if (!kindLabel) {
        if (normalizedKind === 'group') {
          const normalizedGroupName = (groupName ?? label).trim().toLowerCase()
          kindLabel = normalizedGroupName.includes('admin') ? 'Admins' : 'Group'
        } else if (normalizedKind === 'service') {
          kindLabel = label.trim().toLowerCase().includes('panel') ? 'Panel' : 'Service'
        } else if (normalizedKind === 'project') {
          kindLabel = 'Project'
        } else {
          kindLabel = 'Node'
        }
      }
      if (!tone) {
        if (normalizedKind === 'group') tone = 'group'
        else if (normalizedKind === 'service') tone = 'service'
        else if (normalizedKind === 'project') tone = 'project'
        else tone = 'neutral'
      }

      const normalized: NormalizedMockNetbirdNode = {
        id,
        label,
        kind,
        kindLabel,
        tone,
        ...(groupName ? { groupName } : {}),
        ...(projectName ? { projectName } : {}),
        ...(projectID != null ? { projectId: Math.trunc(projectID) } : {}),
      }
      return normalized
    })
    .filter(isNonNull)

  const nodeLabelByID = new Map<string, string>(
    nodes.map((node) => [node.id, node.label]),
  )

  const rawEdges = Array.isArray(rawGraph.edges) ? rawGraph.edges : []
  const edges = rawEdges
    .map((rawEdge, index) => {
      const edge = objectOrNull(rawEdge)
      if (!edge) return null

      const from = stringOrUndefined(edge.from)
      const to = stringOrUndefined(edge.to)
      if (!from || !to) return null

      const policy = stringOrUndefined(edge.policy) ?? 'policy'
      const rule = stringOrUndefined(edge.rule) ?? 'rule'
      const action = stringOrUndefined(edge.action) ?? 'accept'
      const protocol = stringOrUndefined(edge.protocol) ?? 'tcp'
      const bidirectional = Boolean(edge.bidirectional)
      const ports = Array.isArray(edge.ports)
        ? edge.ports
            .map((entry) => stringOrUndefined(entry))
            .filter((entry): entry is string => Boolean(entry))
        : []
      const portLabel = ports.length > 0 ? ports.join(', ') : 'any'
      const tone =
        stringOrUndefined(edge.tone) ??
        (['accept', 'allow'].includes(action.trim().toLowerCase()) ? 'allow' : 'neutral')
      const fromLabel = stringOrUndefined(edge.fromLabel) ?? nodeLabelByID.get(from) ?? from
      const toLabel = stringOrUndefined(edge.toLabel) ?? nodeLabelByID.get(to) ?? to
      const ruleLabel =
        stringOrUndefined(edge.ruleLabel) ??
        `${policy}/${rule} · ${protocol.trim().toUpperCase()} ${portLabel}`

      const normalized: NormalizedMockNetbirdEdge = {
        id: stringOrUndefined(edge.id) ?? `edge-${index + 1}:${from}:${to}`,
        from,
        to,
        fromLabel,
        toLabel,
        policy,
        rule,
        ruleLabel,
        action,
        protocol,
        ports,
        bidirectional,
        tone,
      }
      return normalized
    })
    .filter(isNonNull)

  const allowEdgeCount = edges.filter((edge) => {
    const action = String(edge.action || '')
    const normalized = action.trim().toLowerCase()
    return normalized === 'accept' || normalized === 'allow'
  }).length

  const summary = objectOrNull(rawGraph.summary)
  const nodeCount = Math.trunc(numberOrUndefined(summary?.nodeCount) ?? nodes.length)
  const edgeCount = Math.trunc(numberOrUndefined(summary?.edgeCount) ?? edges.length)
  const normalizedAllowEdgeCount = Math.trunc(
    numberOrUndefined(summary?.allowEdgeCount) ?? allowEdgeCount,
  )

  const effectiveModeBProjectIds = Array.isArray(rawGraph.effectiveModeBProjectIds)
    ? rawGraph.effectiveModeBProjectIds
        .map((entry) => numberOrUndefined(entry))
        .filter((entry): entry is number => Number.isFinite(entry))
        .map((entry) => Math.trunc(entry))
    : []
  const configuredModeBProjectIds = Array.isArray(rawGraph.configuredModeBProjectIds)
    ? rawGraph.configuredModeBProjectIds
        .map((entry) => numberOrUndefined(entry))
        .filter((entry): entry is number => Number.isFinite(entry))
        .map((entry) => Math.trunc(entry))
    : []
  const notes = Array.isArray(rawGraph.notes)
    ? rawGraph.notes
        .map((entry) => stringOrUndefined(entry))
        .filter((entry): entry is string => Boolean(entry))
    : []

  return {
    graph: {
      currentMode: normalizedCurrentMode,
      modeLabel:
        stringOrUndefined(rawGraph.modeLabel) ?? normalizeMockNetbirdModeLabel(normalizedCurrentMode),
      configuredMode: normalizedConfiguredMode,
      configuredModeLabel:
        stringOrUndefined(rawGraph.configuredModeLabel) ??
        normalizeMockNetbirdModeLabel(normalizedConfiguredMode),
      effectiveModeBProjectIds,
      configuredModeBProjectIds,
      modeSource: stringOrUndefined(rawGraph.modeSource) ?? 'mock',
      ...(numberOrUndefined(rawGraph.modeSourceJobId) != null
        ? { modeSourceJobId: Math.trunc(numberOrUndefined(rawGraph.modeSourceJobId) ?? 0) }
        : {}),
      modeDrift: Boolean(rawGraph.modeDrift),
      defaultAction,
      defaultActionLabel:
        stringOrUndefined(rawGraph.defaultActionLabel) ??
        normalizeMockNetbirdDefaultActionLabel(defaultAction),
      defaultActionTone:
        stringOrUndefined(rawGraph.defaultActionTone) ??
        normalizeMockNetbirdDefaultActionTone(defaultAction),
      summary: {
        nodeCount,
        edgeCount,
        allowEdgeCount: normalizedAllowEdgeCount,
      },
      nodes,
      edges,
      notes,
    },
  }
}

function resolveDynamicMockResponse(
  method: string,
  url: string | null | undefined,
  data: unknown,
): unknown | undefined {
  const pathname = normalizeMockPath(url)
  if (!pathname) return undefined

  const normalizedMethod = method.toUpperCase()
  if (normalizedMethod === 'GET') {
    const snapshotMatch = pathname.match(workbenchSnapshotPathPattern)
    if (snapshotMatch) {
      const projectSegment = snapshotMatch[1]
      if (!projectSegment) return undefined
      const projectName = decodeProjectSegment(projectSegment)
      const state = getMockWorkbenchState(projectName)
      if (!state) return undefined
      return { stack: cloneMockValue(state.current) }
    }

    const graphMatch = pathname.match(workbenchGraphPathPattern)
    if (graphMatch) {
      const projectSegment = graphMatch[1]
      if (!projectSegment) return undefined
      const projectName = decodeProjectSegment(projectSegment)
      return resolveMockWorkbenchGraph(projectName) ?? undefined
    }
  }

  if (normalizedMethod !== 'POST') return undefined

  const payload = parseMockPayload(data)
  const previewMatch = pathname.match(workbenchComposePreviewPathPattern)
  if (previewMatch) {
    const projectSegment = previewMatch[1]
    if (!projectSegment) return undefined
    const projectName = decodeProjectSegment(projectSegment)
    return resolveMockWorkbenchComposePreview(projectName) ?? undefined
  }

  const mutateMatch = pathname.match(workbenchPortMutatePathPattern)
  if (mutateMatch) {
    const projectSegment = mutateMatch[1]
    if (!projectSegment) return undefined
    const projectName = decodeProjectSegment(projectSegment)
    return resolveMockWorkbenchPortMutation(projectName, payload) ?? undefined
  }

  const suggestMatch = pathname.match(workbenchPortSuggestPathPattern)
  if (suggestMatch) {
    const projectSegment = suggestMatch[1]
    if (!projectSegment) return undefined
    const projectName = decodeProjectSegment(projectSegment)
    return resolveMockWorkbenchPortSuggestions(projectName, payload) ?? undefined
  }

  return undefined
}

api.interceptors.request.use((config) => {
  if (!isMockEnabled()) return config

  const method = config.method || 'get'
  const dynamicMockResponse = resolveDynamicMockResponse(method, config.url, config.data)
  if (dynamicMockResponse !== undefined) {
    config.adapter = async () => ({
      data: dynamicMockResponse,
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })

    return config
  }

  const key = getMockKey(method, config.url)
  if (!key) return config

  let mockResponse = (mockData as Record<string, unknown>)[key]
  if (!mockResponse && key === 'GET /api/v1/netbird/graph') {
    mockResponse = (mockData as Record<string, unknown>)['GET /api/v1/netbird/acl/graph']
  }
  if (key === 'GET /api/v1/netbird/graph' && mockResponse) {
    mockResponse = normalizeMockNetbirdGraphResponse(mockResponse)
  }
  if (!mockResponse) return config

  config.adapter = async () => ({
    data: mockResponse,
    status: 200,
    statusText: 'OK',
    headers: {},
    config,
  })

  return config
})

api.interceptors.response.use(
  (response) => response,
  (error) => Promise.reject(error),
)

export function parseApiError(error: unknown): ApiError {
  if (error instanceof ApiError) {
    return error
  }

  if (axios.isAxiosError(error)) {
    const data =
      typeof error.response?.data === 'object' && error.response?.data !== null
        ? (error.response?.data as Record<string, unknown>)
        : {}

    const nested =
      typeof data.error === 'object' && data.error !== null
        ? (data.error as Record<string, unknown>)
        : {}

    const payload: Record<string, unknown> = { ...data, ...nested }

    const code = stringOrUndefined(payload.code)
    const docsUrl = stringOrUndefined(payload.docsUrl)
    const fields = recordOrUndefined(payload.fields)
    const details = payload.details

    const message =
      stringOrUndefined(payload.message) ||
      (typeof data.error === 'string' ? data.error : undefined) ||
      error.message ||
      'Unexpected error'

    return new ApiError(message, { code, docsUrl, fields, details })
  }

  if (error instanceof Error) {
    return new ApiError(error.message)
  }

  return new ApiError('Unexpected error')
}

export function apiErrorMessage(error: unknown): string {
  const parsed = parseApiError(error)
  if (parsed.code) {
    return `[${parsed.code}] ${parsed.message}`
  }
  return parsed.message
}

export function apiErrorCode(error: unknown): string | undefined {
  return parseApiError(error).code
}

export function apiErrorDocsUrl(error: unknown): string | undefined {
  return parseApiError(error).docsUrl
}

function stringOrUndefined(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}

function numberOrUndefined(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return undefined
    const parsed = Number(trimmed)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function recordOrUndefined(value: unknown): Record<string, string> | undefined {
  if (!value || typeof value !== 'object') return undefined
  const entries = Object.entries(value as Record<string, unknown>).filter(
    ([, v]) => typeof v === 'string',
  )
  if (entries.length === 0) return undefined
  return Object.fromEntries(entries) as Record<string, string>
}

export function getApiBaseUrl(): string {
  return apiBaseUrl
}

import axios from 'axios'
import mockData from '@/mock.json'
import type { WorkbenchPortSelector, WorkbenchStackPort, WorkbenchStackSnapshot } from '@/types/workbench'

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
  }

  if (normalizedMethod !== 'POST') return undefined

  const payload = parseMockPayload(data)
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

  const mockResponse = (mockData as Record<string, unknown>)[key]
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

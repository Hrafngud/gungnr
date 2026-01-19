import axios from 'axios'
import mockData from '@/mock.json'

export class ApiError extends Error {
  fields?: Record<string, string>

  constructor(message: string, fields?: Record<string, string>) {
    super(message)
    this.name = 'ApiError'
    this.fields = fields
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

api.interceptors.request.use((config) => {
  if (!isMockEnabled()) return config

  const method = config.method || 'get'
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
        ? (error.response?.data as {
            error?: string
            message?: string
            fields?: Record<string, string>
          })
        : {}

    const message = data.error || data.message || error.message || 'Unexpected error'
    return new ApiError(message, data.fields)
  }

  if (error instanceof Error) {
    return new ApiError(error.message)
  }

  return new ApiError('Unexpected error')
}

export function apiErrorMessage(error: unknown): string {
  return parseApiError(error).message
}

export function getApiBaseUrl(): string {
  return apiBaseUrl
}

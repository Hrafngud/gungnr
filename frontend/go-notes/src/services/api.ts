import axios from 'axios'

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
const resolvedBaseUrl = resolveBaseUrl(envBaseUrl, defaultBaseUrl)
const apiBaseUrl = resolvedBaseUrl.replace(/\/$/, '')

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

import axios from 'axios'

export class ApiError extends Error {
  fields?: Record<string, string>

  constructor(message: string, fields?: Record<string, string>) {
    super(message)
    this.name = 'ApiError'
    this.fields = fields
  }
}

const defaultBaseUrl = 'http://localhost:8080/api/v1'

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || defaultBaseUrl,
  headers: {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  },
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

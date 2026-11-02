import axios from 'axios'
import { QueryClient } from '@tanstack/vue-query'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        if (axios.isAxiosError(error) && (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT' || error.code === 'ERR_CANCELED')) return false
        return failureCount < 1
      },
      refetchOnWindowFocus: false,
      staleTime: 15_000,
    },
  },
})

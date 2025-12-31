import { api } from '@/services/api'
import type { OnboardingState, OnboardingUpdate } from '@/types/onboarding'

export const onboardingApi = {
  get: () => api.get<{ state: OnboardingState }>('/api/v1/onboarding'),
  update: (payload: OnboardingUpdate) =>
    api.patch<{ state: OnboardingState }>('/api/v1/onboarding', payload),
}

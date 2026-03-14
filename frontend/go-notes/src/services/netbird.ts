import { api } from '@/services/api'
import type {
  NetBirdACLGraphResponse,
  NetBirdModeConfigResponse,
  NetBirdModeConfigUpdateRequest,
  NetBirdModeApplyRequest,
  NetBirdModeApplyResponse,
  NetBirdModePlanRequest,
  NetBirdModePlanResponse,
  NetBirdPolicyReapplyRequest,
  NetBirdPolicyReapplyResponse,
  NetBirdStatusResponse,
} from '@/types/netbird'

export const netbirdApi = {
  getStatus: () => api.get<NetBirdStatusResponse>('/api/v1/netbird/status'),
  getModeConfig: () => api.get<NetBirdModeConfigResponse>('/api/v1/netbird/config'),
  updateModeConfig: (payload: NetBirdModeConfigUpdateRequest) =>
    api.put<NetBirdModeConfigResponse>('/api/v1/netbird/config', payload),
  planModeSwitch: (payload: NetBirdModePlanRequest) =>
    api.post<NetBirdModePlanResponse>('/api/v1/netbird/mode/plan', payload),
  applyModeSwitch: (payload: NetBirdModeApplyRequest) =>
    api.post<NetBirdModeApplyResponse>('/api/v1/netbird/mode/apply', payload),
  reapplyPolicies: (payload: NetBirdPolicyReapplyRequest = {}) =>
    api.post<NetBirdPolicyReapplyResponse>('/api/v1/netbird/policies/reapply', payload),
  getGraph: () => api.get<NetBirdACLGraphResponse>('/api/v1/netbird/graph'),
}

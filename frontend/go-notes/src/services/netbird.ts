import { api } from '@/services/api'
import type {
  NetBirdACLGraphResponse,
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
  planModeSwitch: (payload: NetBirdModePlanRequest) =>
    api.post<NetBirdModePlanResponse>('/api/v1/netbird/mode/plan', payload),
  applyModeSwitch: (payload: NetBirdModeApplyRequest) =>
    api.post<NetBirdModeApplyResponse>('/api/v1/netbird/mode/apply', payload),
  reapplyPolicies: (payload: NetBirdPolicyReapplyRequest = {}) =>
    api.post<NetBirdPolicyReapplyResponse>('/api/v1/netbird/policies/reapply', payload),
  getAclGraph: () => api.get<NetBirdACLGraphResponse>('/api/v1/netbird/acl/graph'),
}

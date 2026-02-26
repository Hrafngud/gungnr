import type { Job } from '@/types/jobs'

export type NetBirdMode = 'legacy' | 'mode_a' | 'mode_b'

export interface NetBirdOperationCounts {
  created: number
  updated: number
  deleted: number
  unchanged: number
}

export interface NetBirdAPIReachability {
  source: string
  checkedAt?: string
  message?: string
}

export interface NetBirdStatus {
  clientInstalled: boolean
  daemonRunning: boolean
  connected: boolean
  peerId?: string
  peerName?: string
  wg0Ip?: string
  currentMode: NetBirdMode
  lastPolicySyncAt?: string
  lastPolicySyncStatus: string
  lastPolicySyncJobId?: number
  lastPolicySyncError?: string
  lastPolicySyncWarnings: number
  lastGroupResults: NetBirdOperationCounts
  lastPolicyResults: NetBirdOperationCounts
  apiReachable: boolean
  apiReachability: NetBirdAPIReachability
  managedGroups: number
  managedPolicies: number
  managedCountSource: string
  warnings: string[]
}

export interface NetBirdStatusResponse {
  status: NetBirdStatus
}

export interface NetBirdGroupPayload {
  name: string
  peers: string[]
}

export interface NetBirdPolicyRuleSpec {
  name: string
  description?: string
  enabled: boolean
  action: string
  bidirectional: boolean
  protocol: string
  ports: string[]
  sources: string[]
  destinations: string[]
}

export interface NetBirdPolicyPayload {
  name: string
  description?: string
  enabled: boolean
  rules: NetBirdPolicyRuleSpec[]
}

export interface NetBirdCatalog {
  groups: NetBirdGroupPayload[]
  policies: NetBirdPolicyPayload[]
}

export interface NetBirdGroupOperation {
  operation: string
  name: string
  match?: string
  payload?: NetBirdGroupPayload
  reason?: string
}

export interface NetBirdPolicyOperation {
  operation: string
  name: string
  match?: string
  payload?: NetBirdPolicyPayload
  reason?: string
}

export interface NetBirdServiceRebindingOperation {
  service: string
  projectId?: number
  projectName?: string
  port: number
  fromListeners: string[]
  toListeners: string[]
  reason: string
}

export interface NetBirdRedeployProjectTarget {
  projectId: number
  projectName: string
  port: number
  reason: string
}

export interface NetBirdRedeployTargets {
  panel: boolean
  projects: NetBirdRedeployProjectTarget[]
}

export interface NetBirdModePlan {
  currentMode: NetBirdMode
  targetMode: NetBirdMode
  allowLocalhost: boolean
  catalog: NetBirdCatalog
  groupOperations: NetBirdGroupOperation[]
  policyOperations: NetBirdPolicyOperation[]
  serviceRebindingOperations: NetBirdServiceRebindingOperation[]
  redeployTargets: NetBirdRedeployTargets
  warnings: string[]
}

export interface NetBirdModePlanRequest {
  targetMode: NetBirdMode
  allowLocalhost: boolean
}

export interface NetBirdModePlanResponse {
  plan: NetBirdModePlan
}

export interface NetBirdModeApplyRequest {
  targetMode: NetBirdMode
  allowLocalhost: boolean
  apiBaseUrl?: string
  apiToken: string
  hostPeerId?: string
  adminPeerIds?: string[]
}

export interface NetBirdModeApplyResponse {
  job: Job
}

export interface NetBirdReconcileOperation {
  name: string
  resourceId?: string
  result: string
}

export interface NetBirdDefaultPolicySummary {
  action: string
  result: NetBirdReconcileOperation
}

export interface NetBirdPolicyReapplyRequest {
  apiBaseUrl?: string
  apiToken?: string
  hostPeerId?: string
  adminPeerIds?: string[]
}

export interface NetBirdPolicyReapplySummary {
  currentMode: NetBirdMode
  defaultPolicy: NetBirdDefaultPolicySummary
  groupResultCounts: NetBirdOperationCounts
  policyResultCounts: NetBirdOperationCounts
  groupOperations: NetBirdReconcileOperation[]
  policyOperations: NetBirdReconcileOperation[]
  warnings: string[]
}

export interface NetBirdPolicyReapplyResponse {
  summary: NetBirdPolicyReapplySummary
}

export interface NetBirdACLNode {
  id: string
  label: string
  kind: string
  groupName?: string
  projectId?: number
  projectName?: string
}

export interface NetBirdACLEdge {
  id: string
  from: string
  to: string
  policy: string
  rule: string
  action: string
  protocol: string
  ports: string[]
  bidirectional: boolean
}

export interface NetBirdACLGraph {
  currentMode: NetBirdMode
  defaultAction: string
  nodes: NetBirdACLNode[]
  edges: NetBirdACLEdge[]
  notes: string[]
}

export interface NetBirdACLGraphResponse {
  graph: NetBirdACLGraph
}

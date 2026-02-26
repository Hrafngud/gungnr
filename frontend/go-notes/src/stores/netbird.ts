import { ref } from 'vue'
import { defineStore } from 'pinia'
import { netbirdApi } from '@/services/netbird'
import { apiErrorMessage } from '@/services/api'
import type {
  NetBirdACLGraph,
  NetBirdModeApplyRequest,
  NetBirdModePlan,
  NetBirdModePlanRequest,
  NetBirdPolicyReapplyRequest,
  NetBirdPolicyReapplySummary,
  NetBirdStatus,
} from '@/types/netbird'
import type { Job } from '@/types/jobs'

interface NetBirdStatusSlice {
  loading: boolean
  error: string | null
  data: NetBirdStatus | null
}

interface NetBirdModePlanSlice {
  loading: boolean
  error: string | null
  data: NetBirdModePlan | null
}

interface NetBirdModeApplySlice {
  submitting: boolean
  error: string | null
  job: Job | null
}

interface NetBirdPolicyReapplySlice {
  submitting: boolean
  error: string | null
  summary: NetBirdPolicyReapplySummary | null
}

interface NetBirdAclGraphSlice {
  loading: boolean
  error: string | null
  data: NetBirdACLGraph | null
}

export const useNetbirdStore = defineStore('netbird', () => {
  const status = ref<NetBirdStatusSlice>({
    loading: false,
    error: null,
    data: null,
  })
  const modePlan = ref<NetBirdModePlanSlice>({
    loading: false,
    error: null,
    data: null,
  })
  const modeApply = ref<NetBirdModeApplySlice>({
    submitting: false,
    error: null,
    job: null,
  })
  const policyReapply = ref<NetBirdPolicyReapplySlice>({
    submitting: false,
    error: null,
    summary: null,
  })
  const aclGraph = ref<NetBirdAclGraphSlice>({
    loading: false,
    error: null,
    data: null,
  })

  async function loadStatus() {
    if (status.value.loading) return
    status.value.loading = true
    status.value.error = null
    try {
      const { data } = await netbirdApi.getStatus()
      status.value.data = data.status
    } catch (err: unknown) {
      status.value.error = apiErrorMessage(err)
      status.value.data = null
    } finally {
      status.value.loading = false
    }
  }

  async function loadAclGraph() {
    if (aclGraph.value.loading) return
    aclGraph.value.loading = true
    aclGraph.value.error = null
    try {
      const { data } = await netbirdApi.getAclGraph()
      aclGraph.value.data = data.graph
    } catch (err: unknown) {
      aclGraph.value.error = apiErrorMessage(err)
      aclGraph.value.data = null
    } finally {
      aclGraph.value.loading = false
    }
  }

  async function planModeSwitch(payload: NetBirdModePlanRequest) {
    if (modePlan.value.loading) return
    modePlan.value.loading = true
    modePlan.value.error = null
    try {
      const { data } = await netbirdApi.planModeSwitch(payload)
      modePlan.value.data = data.plan
    } catch (err: unknown) {
      modePlan.value.error = apiErrorMessage(err)
      modePlan.value.data = null
    } finally {
      modePlan.value.loading = false
    }
  }

  async function applyModeSwitch(payload: NetBirdModeApplyRequest) {
    if (modeApply.value.submitting) return
    modeApply.value.submitting = true
    modeApply.value.error = null
    modeApply.value.job = null
    try {
      const { data } = await netbirdApi.applyModeSwitch(payload)
      modeApply.value.job = data.job
    } catch (err: unknown) {
      modeApply.value.error = apiErrorMessage(err)
    } finally {
      modeApply.value.submitting = false
    }
  }

  async function reapplyPolicies(payload: NetBirdPolicyReapplyRequest = {}) {
    if (policyReapply.value.submitting) return
    policyReapply.value.submitting = true
    policyReapply.value.error = null
    policyReapply.value.summary = null
    try {
      const { data } = await netbirdApi.reapplyPolicies(payload)
      policyReapply.value.summary = data.summary
    } catch (err: unknown) {
      policyReapply.value.error = apiErrorMessage(err)
    } finally {
      policyReapply.value.submitting = false
    }
  }

  return {
    status,
    modePlan,
    modeApply,
    policyReapply,
    aclGraph,
    loadStatus,
    loadAclGraph,
    planModeSwitch,
    applyModeSwitch,
    reapplyPolicies,
  }
})

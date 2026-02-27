import { ref } from 'vue'
import { defineStore } from 'pinia'
import { apiErrorMessage } from '@/services/api'
import { jobsApi } from '@/services/jobs'
import { netbirdApi } from '@/services/netbird'
import type {
  NetBirdACLGraph,
  NetBirdModeConfig,
  NetBirdModeConfigUpdateRequest,
  NetBirdModeApplyRequest,
  NetBirdModePlan,
  NetBirdModePlanRequest,
  NetBirdPolicyReapplyRequest,
  NetBirdPolicyReapplySummary,
  NetBirdStatus,
} from '@/types/netbird'
import type { Job, JobDetail } from '@/types/jobs'

const MODE_APPLY_POLL_INTERVAL_MS = 2000
const MODE_APPLY_MAX_ATTEMPTS = 180
const NETBIRD_MODE_APPLY_SUMMARY_PREFIX = 'netbird_mode_apply_summary='

type NetBirdModeApplyPollingLifecycle = 'idle' | 'running' | 'terminal' | 'error'

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

interface NetBirdModeConfigSlice {
  loading: boolean
  error: string | null
  saving: boolean
  saveError: string | null
  data: NetBirdModeConfig | null
}

interface NetBirdModeApplyExecutionCounts {
  succeeded: number
  failed: number
  skipped: number
}

interface NetBirdModeApplyExecutionSummary {
  counts?: Partial<NetBirdModeApplyExecutionCounts>
}

interface NetBirdModeApplySummary {
  warnings?: string[]
  rebindingExecution?: NetBirdModeApplyExecutionSummary
  redeployExecution?: NetBirdModeApplyExecutionSummary
}

interface NetBirdModeApplyPollingSlice {
  lifecycle: NetBirdModeApplyPollingLifecycle
  error: string | null
  jobId: number | null
  lastJob: JobDetail | null
  summary: NetBirdModeApplySummary | null
  attempts: number
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

const createModeApplyPollingState = (): NetBirdModeApplyPollingSlice => ({
  lifecycle: 'idle',
  error: null,
  jobId: null,
  lastJob: null,
  summary: null,
  attempts: 0,
})

const isTerminalJobStatus = (status?: string): boolean => {
  const normalized = (status || '').trim().toLowerCase()
  return normalized === 'completed' || normalized === 'failed'
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  Boolean(value) && typeof value === 'object' && !Array.isArray(value)

const parseModeApplySummaryFromLogLines = (logLines?: string[]): NetBirdModeApplySummary | null => {
  if (!Array.isArray(logLines) || logLines.length === 0) return null

  for (let index = logLines.length - 1; index >= 0; index -= 1) {
    const line = (logLines[index] || '').trim()
    if (!line.startsWith(NETBIRD_MODE_APPLY_SUMMARY_PREFIX)) continue

    const rawPayload = line.slice(NETBIRD_MODE_APPLY_SUMMARY_PREFIX.length).trim()
    if (!rawPayload) return null

    try {
      const parsed = JSON.parse(rawPayload)
      if (!isRecord(parsed)) return null
      return parsed as NetBirdModeApplySummary
    } catch {
      return null
    }
  }

  return null
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
  const modeConfig = ref<NetBirdModeConfigSlice>({
    loading: false,
    error: null,
    saving: false,
    saveError: null,
    data: null,
  })
  const modeApplyPolling = ref<NetBirdModeApplyPollingSlice>(createModeApplyPollingState())
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
  let modeApplyPollTimer: ReturnType<typeof setTimeout> | null = null
  let modeApplyPollSession = 0

  const clearModeApplyPollTimer = () => {
    if (modeApplyPollTimer !== null) {
      clearTimeout(modeApplyPollTimer)
      modeApplyPollTimer = null
    }
  }

  const applyModeApplyPollingSnapshot = (job: JobDetail | null) => {
    modeApplyPolling.value.lastJob = job
    modeApplyPolling.value.summary = parseModeApplySummaryFromLogLines(job?.logLines)
  }

  const scheduleModeApplyPoll = (sessionID: number) => {
    clearModeApplyPollTimer()
    modeApplyPollTimer = setTimeout(() => {
      void pollModeApplyJob(sessionID)
    }, MODE_APPLY_POLL_INTERVAL_MS)
  }

  const markModeApplyPollingError = (message: string) => {
    modeApplyPolling.value.lifecycle = 'error'
    modeApplyPolling.value.error = message
    clearModeApplyPollTimer()
  }

  const pollModeApplyJob = async (sessionID: number) => {
    if (sessionID !== modeApplyPollSession) return

    const jobId = modeApplyPolling.value.jobId
    if (!jobId) return

    try {
      const { data } = await jobsApi.get(jobId)
      if (sessionID !== modeApplyPollSession) return

      modeApplyPolling.value.error = null
      modeApplyPolling.value.attempts += 1
      applyModeApplyPollingSnapshot(data)

      if (isTerminalJobStatus(data.status)) {
        modeApplyPolling.value.lifecycle = 'terminal'
        clearModeApplyPollTimer()
        return
      }

      if (modeApplyPolling.value.attempts >= MODE_APPLY_MAX_ATTEMPTS) {
        markModeApplyPollingError(
          `Mode apply polling timed out after ${MODE_APPLY_MAX_ATTEMPTS} attempts.`,
        )
        return
      }

      modeApplyPolling.value.lifecycle = 'running'
      scheduleModeApplyPoll(sessionID)
    } catch (err: unknown) {
      if (sessionID !== modeApplyPollSession) return
      markModeApplyPollingError(apiErrorMessage(err))
    }
  }

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

  async function loadModeConfig() {
    if (modeConfig.value.loading) return
    modeConfig.value.loading = true
    modeConfig.value.error = null
    try {
      const { data } = await netbirdApi.getModeConfig()
      modeConfig.value.data = data.config
    } catch (err: unknown) {
      modeConfig.value.error = apiErrorMessage(err)
      modeConfig.value.data = null
    } finally {
      modeConfig.value.loading = false
    }
  }

  async function saveModeConfig(payload: NetBirdModeConfigUpdateRequest) {
    if (modeConfig.value.saving) return
    modeConfig.value.saving = true
    modeConfig.value.saveError = null
    try {
      const { data } = await netbirdApi.updateModeConfig(payload)
      modeConfig.value.data = data.config
    } catch (err: unknown) {
      modeConfig.value.saveError = apiErrorMessage(err)
    } finally {
      modeConfig.value.saving = false
    }
  }

  function stopModeApplyJobPolling(reset = false) {
    clearModeApplyPollTimer()
    modeApplyPollSession += 1

    if (reset) {
      modeApplyPolling.value = createModeApplyPollingState()
      return
    }

    if (modeApplyPolling.value.lifecycle === 'running') {
      modeApplyPolling.value.lifecycle = 'idle'
    }
  }

  async function startModeApplyJobPolling(jobId: number) {
    if (!Number.isFinite(jobId) || jobId <= 0) return

    const normalizedJobId = Math.trunc(jobId)
    if (
      modeApplyPolling.value.jobId === normalizedJobId &&
      modeApplyPolling.value.lifecycle === 'running'
    ) {
      return
    }

    clearModeApplyPollTimer()
    modeApplyPollSession += 1
    const sessionID = modeApplyPollSession
    const previousSnapshot =
      modeApplyPolling.value.lastJob && modeApplyPolling.value.lastJob.id === normalizedJobId
        ? modeApplyPolling.value.lastJob
        : null
    modeApplyPolling.value = {
      lifecycle: 'running',
      error: null,
      jobId: normalizedJobId,
      lastJob: previousSnapshot,
      summary: parseModeApplySummaryFromLogLines(previousSnapshot?.logLines),
      attempts: 0,
    }
    await pollModeApplyJob(sessionID)
  }

  async function applyModeSwitch(payload: NetBirdModeApplyRequest) {
    if (modeApply.value.submitting) return
    modeApply.value.submitting = true
    modeApply.value.error = null
    modeApply.value.job = null
    stopModeApplyJobPolling(true)
    try {
      const { data } = await netbirdApi.applyModeSwitch(payload)
      modeApply.value.job = data.job
      void startModeApplyJobPolling(data.job.id)
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
    modeConfig,
    modeApplyPolling,
    policyReapply,
    aclGraph,
    loadStatus,
    loadModeConfig,
    loadAclGraph,
    planModeSwitch,
    saveModeConfig,
    applyModeSwitch,
    startModeApplyJobPolling,
    stopModeApplyJobPolling,
    reapplyPolicies,
  }
})

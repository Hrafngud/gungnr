import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { jobsApi } from '@/services/jobs'
import { apiErrorMessage } from '@/services/api'
import type { HostDeployResponse, JobDetail } from '@/types/jobs'

const buildCommand = (token: string): string => {
  const origin =
    typeof window !== 'undefined' && window.location.origin
      ? window.location.origin
      : 'http://localhost'
  return `./deploy.sh worker --token ${token} --api ${origin}`
}

const isJobDone = (status?: string) => status === 'completed' || status === 'failed'

export const useHostWorker = () => {
  const modalOpen = ref(false)
  const job = ref<JobDetail | null>(null)
  const logs = ref<string[]>([])
  const error = ref<string | null>(null)
  const action = ref('')
  const expiresAt = ref<string | null>(null)
  const token = ref('')
  const polling = ref(false)
  const command = computed(() => (token.value ? buildCommand(token.value) : ''))

  let pollTimer: ReturnType<typeof setTimeout> | null = null
  let pollInFlight = false

  const stopPolling = () => {
    if (pollTimer) {
      clearTimeout(pollTimer)
      pollTimer = null
    }
    polling.value = false
  }

  const schedulePoll = () => {
    pollTimer = setTimeout(() => {
      void pollJob()
    }, 3000)
  }

  const pollJob = async () => {
    if (!job.value || pollInFlight) return
    pollInFlight = true
    try {
      const { data } = await jobsApi.get(job.value.id)
      job.value = data
      logs.value = data.logLines ?? []
      error.value = null
      if (isJobDone(data.status)) {
        stopPolling()
        return
      }
    } catch (err) {
      error.value = apiErrorMessage(err)
    } finally {
      pollInFlight = false
    }
    if (modalOpen.value) {
      schedulePoll()
    }
  }

  const openWithHostDeploy = async (response: HostDeployResponse) => {
    stopPolling()
    job.value = { ...response.job, logLines: [] }
    logs.value = []
    action.value = response.action
    token.value = response.token
    expiresAt.value = response.expiresAt ?? null
    error.value = null
    modalOpen.value = true
    polling.value = true
    await pollJob()
  }

  const closeModal = () => {
    modalOpen.value = false
    stopPolling()
  }

  onBeforeUnmount(() => {
    stopPolling()
  })

  watch(modalOpen, (open) => {
    if (!open) {
      stopPolling()
    }
  })

  return {
    modalOpen,
    job,
    logs,
    error,
    action,
    expiresAt,
    token,
    polling,
    command,
    openWithHostDeploy,
    closeModal,
  }
}

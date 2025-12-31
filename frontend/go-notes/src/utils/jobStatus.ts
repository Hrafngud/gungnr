export type JobBadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

export const jobStatusTone = (status?: string): JobBadgeTone => {
  switch ((status || '').toLowerCase()) {
    case 'completed':
      return 'ok'
    case 'running':
      return 'warn'
    case 'failed':
      return 'error'
    default:
      return 'neutral'
  }
}

export const jobStatusLabel = (status?: string): string => {
  switch ((status || '').toLowerCase()) {
    case 'pending':
      return 'queued'
    case 'running':
      return 'running'
    case 'completed':
      return 'completed'
    case 'failed':
      return 'failed'
    case '':
      return 'pending'
    default:
      return (status || '').replace(/_/g, ' ')
  }
}

export const isPendingJob = (status?: string): boolean =>
  status === 'pending'

export const jobActionLabel = (action?: string): string => {
  switch ((action || '').toLowerCase()) {
    case 'create_template':
      return 'Create template'
    case 'deploy_existing':
      return 'Deploy existing'
    case 'quick_service':
      return 'Quick service'
    default:
      return action ? action.replace(/_/g, ' ') : 'Deploy'
  }
}

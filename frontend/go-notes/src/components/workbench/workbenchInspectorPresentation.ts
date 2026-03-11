import type { BadgeTone } from '@/components/workbench/projectDetailWorkbenchTypes'

export interface WorkbenchPortSuggestion {
  rank: number
  hostPort: number
}

export function workbenchCompactToneClass(tone: BadgeTone | undefined): string {
  switch (tone) {
    case 'ok':
      return 'workbench-compact-status-ok'
    case 'warn':
      return 'workbench-compact-status-warn'
    case 'error':
      return 'workbench-compact-status-error'
    default:
      return 'workbench-compact-status-neutral'
  }
}

export function workbenchGuidanceToneClass(status: string): string {
  if (status === 'unavailable') return 'text-[color:var(--danger)]'
  if (status === 'conflict') return 'text-[color:var(--warn)]'
  return 'text-[color:var(--muted)]'
}

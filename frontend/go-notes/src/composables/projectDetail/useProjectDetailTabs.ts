export const PROJECT_DETAIL_SECTION_TABS = [
  {
    id: 'workbench',
    label: 'Workbench',
    description: 'Model snapshot, service controls, and compose workflow.',
  },
  {
    id: 'runtime',
    label: 'Runtime units',
    description: 'Live container inventory tied to this project.',
  },
  {
    id: 'archive',
    label: 'Plan and execute',
    description: 'Archive cleanup planning and queue execution.',
  },
  {
    id: 'activity',
    label: 'Activity timeline',
    description: 'Project-scoped jobs and log inspection.',
  },
] as const

export type ProjectDetailSectionTab = (typeof PROJECT_DETAIL_SECTION_TABS)[number]['id']

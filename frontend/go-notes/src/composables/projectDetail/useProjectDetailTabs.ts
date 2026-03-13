export const PROJECT_DETAIL_SECTION_TABS = [
  {
    id: 'workbench',
    label: 'Workbench',
    description: 'Tweak your Compose files from UI!',
  },
  {
    id: 'runtime',
    label: 'Runtime units',
    description: 'Spy on your runtime units.',
  },
  {
    id: 'archive',
    label: 'Archive',
    description: 'Drop the project for later.',
  },
  {
    id: 'activity',
    label: 'Activity timeline',
    description: 'Boring logs I hope you never have to read.',
  },
] as const

export type ProjectDetailSectionTab = (typeof PROJECT_DETAIL_SECTION_TABS)[number]['id']

import type { IconName } from '@/components/NavIcon.vue'

export const PROJECT_DETAIL_SECTION_TABS = [
  {
    id: 'workbench',
    label: 'Workbench',
    icon: 'overview',
    description: 'Tweak your Compose files from UI!',
  },
  {
    id: 'runtime',
    label: 'Runtime units',
    icon: 'host',
    description: 'Spy on your runtime units.',
  },
  {
    id: 'environment',
    label: 'Environment',
    icon: 'edit',
    description: 'Edit project environment variables.',
  },
  {
    id: 'archive',
    label: 'Archive',
    icon: 'template',
    description: 'Drop the project for later.',
  },
  {
    id: 'activity',
    label: 'Activity timeline',
    icon: 'activity',
    description: 'Boring logs I hope you never have to read.',
  },
] as const satisfies readonly {
  id: string
  label: string
  icon: IconName
  description: string
}[]

export type ProjectDetailSectionTab = (typeof PROJECT_DETAIL_SECTION_TABS)[number]['id']

<script setup lang="ts">
import { computed } from 'vue'

type IconName =
  | 'home'
  | 'overview'
  | 'host'
  | 'network'
  | 'github'
  | 'activity'
  | 'logs'
  | 'users'
  | 'arrow-left'
  | 'arrow-right'
  | 'refresh'
  | 'edit'
  | 'login'
  | 'logout'
  | 'stop'
  | 'restart'
  | 'trash'
  | 'template'
  | 'forward'
  | 'eye'
  | 'eye-off'

const props = defineProps<{
  name: IconName
  class?: string
}>()

const fontawesomeIcons: Partial<Record<IconName, [string, string]>> = {
  github: ['fab', 'github'],
}

const faIcon = computed(() => fontawesomeIcons[props.name])

const paths: Record<IconName, string[]> = {
  home: [
    'M3 10.5L12 3l9 7.5v9.5a2 2 0 0 1-2 2h-4.5a1 1 0 0 1-1-1v-5h-3v5a1 1 0 0 1-1 1H5a2 2 0 0 1-2-2z',
  ],
  overview: [
    'M4 5h7v7H4z',
    'M13 5h7v4h-7z',
    'M13 11h7v8h-7z',
    'M4 14h7v5H4z',
  ],
  host: [
    'M5 5.5h14v5H5z',
    'M5 13.5h14v5H5z',
    'M8 8h0.01',
    'M8 16h0.01',
  ],
  network: [
    'M12 3v4',
    'M12 17v4',
    'M3 12h4',
    'M17 12h4',
    'M7 7l2 2',
    'M15 7l-2 2',
    'M7 17l2-2',
    'M15 17l-2-2',
    'M12 10a2 2 0 1 1 0 4a2 2 0 0 1 0-4z',
  ],
  github: [
    'M8 7l-5 5l5 5',
    'M16 7l5 5l-5 5',
    'M12 6l-2 12',
  ],
  activity: [
    'M4 12h4l2-6l4 12l2-6h4',
  ],
  logs: [
    'M4 6h16v12H4z',
    'M7 10l2 2l-2 2',
    'M11 14h6',
  ],
  users: [
    'M7 11a3 3 0 1 0 0-6a3 3 0 0 0 0 6z',
    'M17 11a3 3 0 1 0 0-6a3 3 0 0 0 0 6z',
    'M3 20a4 4 0 0 1 8 0',
    'M13 20a4 4 0 0 1 8 0',
  ],
  'arrow-left': [
    'M15 18l-6-6l6-6',
  ],
  'arrow-right': [
    'M9 18l6-6l-6-6',
  ],
  refresh: [
    'M3 2v6h6',
    'M21 12a9 9 0 0 0-15.5-6.5L3 8',
    'M21 22v-6h-6',
    'M3 12a9 9 0 0 0 15.5 6.5L21 16',
  ],
  edit: [
    'M12 20h9',
    'M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1l1-4z',
  ],
  login: [
    'M15 3h4a2 2 0 0 1 2 2v4',
    'M10 17l5-5l-5-5',
    'M15 12H3',
    'M15 21h4a2 2 0 0 0 2-2v-4',
  ],
  logout: [
    'M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4',
    'M16 17l5-5l-5-5',
    'M21 12H9',
  ],
  stop: [
    'M6 6h12v12H6z',
  ],
  restart: [
    'M21 2v6h-6',
    'M3 12a9 9 0 0 1 14.657-6.657L21 8',
    'M3 22v-6h6',
    'M21 12a9 9 0 0 1-14.657 6.657L3 16',
  ],
  trash: [
    'M3 6h18',
    'M8 6V4h8v2',
    'M6 6l1 14h10l1-14',
  ],
  template: [
    'M6 3h8l4 4v10a3 3 0 0 1-3 3H6a3 3 0 0 1-3-3V6a3 3 0 0 1 3-3z',
    'M14 3v4h4',
    'M7 13h10',
    'M7 9h5',
  ],
  forward: [
    'M13 7l4 4l-4 4',
    'M17 11H7',
    'M7 7l-4 4l4 4',
  ],
  eye: [
    'M2 12s3-7 10-7s10 7 10 7s-3 7-10 7s-10-7-10-7z',
    'M12 15a3 3 0 1 0 0-6a3 3 0 0 0 0 6z',
  ],
  'eye-off': [
    'M3 3l18 18',
    'M2 12s3-7 10-7c2.3 0 4.3 0.7 5.9 1.7',
    'M22 12s-3 7-10 7c-2.3 0-4.3-0.7-5.9-1.7',
    'M9.9 9.9a3 3 0 0 0 4.2 4.2',
  ],
}
</script>

<template>
  <FontAwesomeIcon
    v-if="faIcon"
    :icon="faIcon"
    :class="props.class"
    aria-hidden="true"
  />
  <svg
    v-else
    :class="props.class"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.6"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path
      v-for="(path, index) in paths[props.name]"
      :key="`${props.name}-${index}`"
      :d="path"
    />
  </svg>
</template>

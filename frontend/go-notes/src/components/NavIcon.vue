<script setup lang="ts">
import { computed } from 'vue'

type IconName = 'home' | 'overview' | 'host' | 'network' | 'github' | 'activity' | 'logs' | 'arrow-left' | 'arrow-right'

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
  'arrow-left': [
    'M15 18l-6-6l6-6',
  ],
  'arrow-right': [
    'M9 18l6-6l-6-6',
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

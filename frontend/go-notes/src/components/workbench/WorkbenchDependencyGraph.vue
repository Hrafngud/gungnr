<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  serviceName: string
  dependsOn: string[]
  dependedBy: string[]
}>()

function normalizeList(values: string[]): string[] {
  const output: string[] = []
  for (const value of values) {
    const normalized = value.trim()
    if (!normalized || output.includes(normalized)) continue
    output.push(normalized)
  }
  return output
}

const upstream = computed(() => normalizeList(props.dependsOn))
const downstream = computed(() => normalizeList(props.dependedBy))
</script>

<template>
  <div class="dependency-graph">
    <div class="dependency-graph__lane">
      <p class="dependency-graph__label">Depends On</p>
      <div v-if="upstream.length > 0" class="dependency-graph__nodes">
        <span
          v-for="node in upstream"
          :key="`${serviceName}-up-${node}`"
          class="dependency-graph__node"
        >
          {{ node }}
        </span>
      </div>
      <p v-else class="dependency-graph__empty">None</p>
    </div>

    <div class="dependency-graph__center">
      <span class="dependency-graph__node dependency-graph__node--active">{{ serviceName }}</span>
      <p v-if="upstream.length > 0" class="dependency-graph__flow">
        <span v-for="node in upstream" :key="`${serviceName}-flow-in-${node}`">{{ node }} -&gt; {{ serviceName }}</span>
      </p>
      <p v-if="downstream.length > 0" class="dependency-graph__flow">
        <span v-for="node in downstream" :key="`${serviceName}-flow-out-${node}`">{{ serviceName }} -&gt; {{ node }}</span>
      </p>
    </div>

    <div class="dependency-graph__lane">
      <p class="dependency-graph__label">Required By</p>
      <div v-if="downstream.length > 0" class="dependency-graph__nodes">
        <span
          v-for="node in downstream"
          :key="`${serviceName}-down-${node}`"
          class="dependency-graph__node"
        >
          {{ node }}
        </span>
      </div>
      <p v-else class="dependency-graph__empty">None</p>
    </div>
  </div>
</template>

<style scoped>
.dependency-graph {
  display: grid;
  gap: 0.75rem;
  border: 1px solid color-mix(in srgb, var(--line) 82%, transparent);
  border-radius: 1rem;
  padding: 0.75rem;
  background-color: color-mix(in srgb, var(--panel) 70%, transparent);
  background-image:
    linear-gradient(
      to right,
      color-mix(in srgb, var(--line) 35%, transparent) 1px,
      transparent 1px
    ),
    linear-gradient(
      to bottom,
      color-mix(in srgb, var(--line) 35%, transparent) 1px,
      transparent 1px
    );
  background-size: 16px 16px;
}

.dependency-graph__lane {
  display: grid;
  gap: 0.45rem;
}

.dependency-graph__center {
  display: grid;
  gap: 0.45rem;
  justify-items: center;
  text-align: center;
}

.dependency-graph__label {
  font-size: 0.65rem;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--muted-2);
}

.dependency-graph__nodes {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.dependency-graph__node {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--line) 82%, transparent);
  background: color-mix(in srgb, var(--panel) 78%, transparent);
  color: var(--text);
  padding: 0.2rem 0.55rem;
  font-size: 0.72rem;
  line-height: 1.2;
}

.dependency-graph__node--active {
  border-color: color-mix(in srgb, var(--accent) 70%, var(--line));
  background: color-mix(in srgb, var(--accent) 15%, var(--panel));
  font-weight: 600;
}

.dependency-graph__flow {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  color: var(--muted);
  font-size: 0.68rem;
}

.dependency-graph__empty {
  color: var(--muted);
  font-size: 0.72rem;
}

@media (min-width: 900px) {
  .dependency-graph {
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
    align-items: start;
  }

  .dependency-graph__center {
    min-width: 12rem;
    padding-top: 1rem;
  }
}
</style>

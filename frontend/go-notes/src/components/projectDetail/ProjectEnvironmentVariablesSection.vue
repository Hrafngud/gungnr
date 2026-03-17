<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import NavIcon from '@/components/NavIcon.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import { apiErrorMessage } from '@/services/api'
import { projectsApi } from '@/services/projects'
import { useToastStore } from '@/stores/toasts'
import type { ProjectEnvRead } from '@/types/projects'

type EnvVariableSource = 'file' | 'resolved' | 'manual'

interface EditableEnvVariable {
  name: string
  value: string
  initialValue: string
  source: EnvVariableSource
  lineIndex: number | null
  exportPrefix: boolean
}

interface ParsedEnvResult {
  lines: string[]
  trailingNewline: boolean
  normalizedContent: string
  variables: EditableEnvVariable[]
}

const props = defineProps<{
  projectName: string
  envPath: string
  envExists: boolean
  isAdmin: boolean
  resolvedVariableNames?: string[]
}>()

const toastStore = useToastStore()

const loading = ref(false)
const saving = ref(false)
const loadError = ref<string | null>(null)
const saveError = ref<string | null>(null)
const saveSuccess = ref<string | null>(null)
const envMeta = ref<ProjectEnvRead | null>(null)
const createBackup = ref(true)
const variables = ref<EditableEnvVariable[]>([])
const sourceLines = ref<string[]>([])
const sourceTrailingNewline = ref(false)
const baselineContent = ref('')
const customVariableName = ref('')
const customVariableValue = ref('')
const customVariableError = ref<string | null>(null)

let latestLoadRequest = 0

const envVariableNamePattern = /^[A-Za-z_][A-Za-z0-9_]*$/
const envAssignmentPattern = /^\s*(export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=(.*)$/
const safeBareValuePattern = /^[A-Za-z0-9_./:@%+=,-]+$/

const resolvedVariableSet = computed(() => {
  const names = props.resolvedVariableNames ?? []
  const set = new Set<string>()
  names.forEach((candidate) => {
    const normalized = candidate.trim()
    if (!normalized || !envVariableNamePattern.test(normalized)) return
    set.add(normalized)
  })
  return set
})

const fileVariableCount = computed(
  () => variables.value.filter((variable) => variable.source === 'file').length,
)
const resolvedOnlyVariableCount = computed(
  () => variables.value.filter((variable) => variable.source === 'resolved').length,
)
const manualVariableCount = computed(
  () => variables.value.filter((variable) => variable.source === 'manual').length,
)

const generatedContent = computed(() =>
  buildEnvContent(variables.value, sourceLines.value, sourceTrailingNewline.value),
)

const hasUnsavedChanges = computed(() => generatedContent.value !== baselineContent.value)
const canSave = computed(() =>
  props.isAdmin &&
  !loading.value &&
  !saving.value &&
  Boolean(props.projectName.trim()) &&
  hasUnsavedChanges.value,
)

const canAddCustomVariable = computed(() =>
  props.isAdmin && !loading.value && !saving.value,
)

const envPathLabel = computed(() => {
  const fromApi = envMeta.value?.path?.trim()
  if (fromApi) return fromApi
  const fromDetail = props.envPath?.trim()
  if (fromDetail) return fromDetail
  return 'unknown'
})

const envExistsLabel = computed(() => {
  if (envMeta.value) return envMeta.value.exists ? 'Present' : 'Missing'
  return props.envExists ? 'Present' : 'Missing'
})

const envExistsTone = computed<'ok' | 'warn'>(() => (
  envExistsLabel.value === 'Present' ? 'ok' : 'warn'
))

const envSizeLabel = computed(() => {
  const sizeBytes = envMeta.value?.sizeBytes
  if (typeof sizeBytes !== 'number') return 'unknown'
  return `${sizeBytes} bytes`
})

const lastUpdatedLabel = computed(() => {
  const updatedAt = envMeta.value?.updatedAt
  if (!updatedAt) return 'unknown'
  const date = new Date(updatedAt)
  if (Number.isNaN(date.getTime())) return updatedAt
  return date.toLocaleString()
})

function normalizeEnvContent(content: string): {
  lines: string[]
  trailingNewline: boolean
  normalizedContent: string
} {
  const lfContent = content.replace(/\r\n/g, '\n')
  const trailingNewline = lfContent.endsWith('\n')
  const body = trailingNewline ? lfContent.slice(0, -1) : lfContent
  const lines = body.length > 0 ? body.split('\n') : []
  const normalizedContent = lines.join('\n') + (trailingNewline ? '\n' : '')
  return {
    lines,
    trailingNewline,
    normalizedContent,
  }
}

function decodeInputValue(rawValue: string): string {
  const trimmed = rawValue.trim()
  if (trimmed.length >= 2 && trimmed.startsWith('"') && trimmed.endsWith('"')) {
    return trimmed
      .slice(1, -1)
      .replace(/\\\\/g, '\\')
      .replace(/\\"/g, '"')
  }
  if (trimmed.length >= 2 && trimmed.startsWith("'") && trimmed.endsWith("'")) {
    return trimmed.slice(1, -1)
  }
  return trimmed
}

function encodeOutputValue(value: string): string {
  if (!value) return ''
  if (safeBareValuePattern.test(value)) return value
  const escaped = value
    .replace(/\\/g, '\\\\')
    .replace(/"/g, '\\"')
  return `"${escaped}"`
}

function parseEnvVariables(
  content: string,
  resolvedNames: Set<string>,
): ParsedEnvResult {
  const normalized = normalizeEnvContent(content)
  const order: string[] = []
  const byName = new Map<string, EditableEnvVariable>()

  normalized.lines.forEach((line, lineIndex) => {
    const match = line.match(envAssignmentPattern)
    if (!match) return

    const variableName = match[2]?.trim() ?? ''
    if (!variableName) return

    const existing = byName.get(variableName)
    const nextValue = decodeInputValue(match[3] ?? '')
    const variable: EditableEnvVariable = {
      name: variableName,
      value: nextValue,
      initialValue: nextValue,
      source: 'file',
      lineIndex,
      exportPrefix: Boolean(match[1]),
    }

    if (!existing) {
      byName.set(variableName, variable)
      order.push(variableName)
      return
    }

    existing.value = variable.value
    existing.initialValue = variable.initialValue
    existing.lineIndex = lineIndex
    existing.exportPrefix = variable.exportPrefix
  })

  const resolvedOnly = Array.from(resolvedNames).filter((name) => !byName.has(name))
  resolvedOnly.sort((left, right) => left.localeCompare(right))

  const fileVariables = order
    .map((name) => byName.get(name))
    .filter((entry): entry is EditableEnvVariable => Boolean(entry))

  const resolvedVariables = resolvedOnly.map((name) => ({
    name,
    value: '',
    initialValue: '',
    source: 'resolved' as const,
    lineIndex: null,
    exportPrefix: false,
  }))

  return {
    lines: normalized.lines,
    trailingNewline: normalized.trailingNewline,
    normalizedContent: normalized.normalizedContent,
    variables: [...fileVariables, ...resolvedVariables],
  }
}

function buildEnvContent(
  currentVariables: EditableEnvVariable[],
  originalLines: string[],
  originalTrailingNewline: boolean,
): string {
  const nextLines = [...originalLines]
  const appendedLines: string[] = []

  currentVariables.forEach((variable) => {
    const variableName = variable.name.trim()
    if (!envVariableNamePattern.test(variableName)) return

    const encodedValue = encodeOutputValue(variable.value)
    if (typeof variable.lineIndex === 'number' && variable.lineIndex >= 0 && variable.lineIndex < nextLines.length) {
      if (variable.value === variable.initialValue) return
      nextLines[variable.lineIndex] = `${variable.exportPrefix ? 'export ' : ''}${variableName}=${encodedValue}`
      return
    }

    if (variable.source === 'resolved' && variable.value.trim() === '') return
    appendedLines.push(`${variableName}=${encodedValue}`)
  })

  const mergedLines = [...nextLines, ...appendedLines]
  if (mergedLines.length === 0) return ''

  const needsTrailingNewline = originalTrailingNewline || appendedLines.length > 0
  return mergedLines.join('\n') + (needsTrailingNewline ? '\n' : '')
}

function applyParsedEnv(parsed: ParsedEnvResult) {
  sourceLines.value = parsed.lines
  sourceTrailingNewline.value = parsed.trailingNewline
  baselineContent.value = parsed.normalizedContent
  variables.value = parsed.variables
}

function syncResolvedVariables() {
  const existing = new Set(variables.value.map((variable) => variable.name))
  const additions: EditableEnvVariable[] = []

  resolvedVariableSet.value.forEach((name) => {
    if (existing.has(name)) return
    additions.push({
      name,
      value: '',
      initialValue: '',
      source: 'resolved',
      lineIndex: null,
      exportPrefix: false,
    })
  })

  if (additions.length === 0) return
  additions.sort((left, right) => left.name.localeCompare(right.name))
  variables.value = [...variables.value, ...additions]
}

async function loadEnv(options?: { preserveSaveFeedback?: boolean }) {
  const name = props.projectName.trim()
  if (!name) return

  const requestId = ++latestLoadRequest
  loading.value = true
  loadError.value = null
  if (!options?.preserveSaveFeedback) {
    saveError.value = null
    saveSuccess.value = null
  }

  try {
    const { data } = await projectsApi.loadEnv(name)
    if (requestId !== latestLoadRequest) return

    envMeta.value = data.env
    const parsed = parseEnvVariables(data.env.content ?? '', resolvedVariableSet.value)
    applyParsedEnv(parsed)
  } catch (err) {
    if (requestId !== latestLoadRequest) return
    envMeta.value = null
    variables.value = []
    sourceLines.value = []
    sourceTrailingNewline.value = false
    baselineContent.value = ''
    loadError.value = apiErrorMessage(err)
  } finally {
    if (requestId === latestLoadRequest) {
      loading.value = false
    }
  }
}

async function saveEnv() {
  const name = props.projectName.trim()
  if (!name || !canSave.value) return

  saving.value = true
  saveError.value = null
  saveSuccess.value = null

  try {
    const payload = generatedContent.value
    const { data } = await projectsApi.saveEnv(name, payload, createBackup.value)
    await loadEnv({ preserveSaveFeedback: true })

    const backupSuffix = data.env.backupPath
      ? ` Backup: ${data.env.backupPath}`
      : ''
    const successMessage = `Environment file updated.${backupSuffix}`
    saveSuccess.value = successMessage
    toastStore.success('Environment variables updated.', 'Project .env')
  } catch (err) {
    const message = apiErrorMessage(err)
    saveError.value = message
    toastStore.error(message, 'Project .env')
  } finally {
    saving.value = false
  }
}

function addCustomVariable() {
  const name = customVariableName.value.trim()
  const value = customVariableValue.value

  customVariableError.value = null
  if (!name) {
    customVariableError.value = 'Variable name is required.'
    return
  }
  if (!envVariableNamePattern.test(name)) {
    customVariableError.value = 'Use letters, numbers, and underscores only.'
    return
  }
  if (variables.value.some((variable) => variable.name === name)) {
    customVariableError.value = 'Variable already exists in this editor.'
    return
  }

  variables.value = [
    ...variables.value,
    {
      name,
      value,
      initialValue: value,
      source: 'manual',
      lineIndex: null,
      exportPrefix: false,
    },
  ]
  customVariableName.value = ''
  customVariableValue.value = ''
}

function removeManualVariable(name: string) {
  variables.value = variables.value.filter((variable) => !(variable.source === 'manual' && variable.name === name))
}

function variableSourceLabel(source: EnvVariableSource): string {
  if (source === 'file') return 'From .env'
  if (source === 'resolved') return 'Resolved'
  return 'Custom'
}

function variableSourceTone(source: EnvVariableSource): 'ok' | 'neutral' {
  return source === 'file' ? 'ok' : 'neutral'
}

watch(
  () => [props.projectName, props.isAdmin],
  () => {
    envMeta.value = null
    variables.value = []
    sourceLines.value = []
    sourceTrailingNewline.value = false
    baselineContent.value = ''
    loadError.value = null
    saveError.value = null
    saveSuccess.value = null
    customVariableName.value = ''
    customVariableValue.value = ''
    customVariableError.value = null

    if (!props.isAdmin) return
    void loadEnv()
  },
  { immediate: true },
)

watch(
  () => props.resolvedVariableNames,
  () => {
    syncResolvedVariables()
  },
  { deep: true },
)
</script>

<template>
  <UiPanel variant="projects" class="space-y-5 p-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project environment</p>
        <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Environment variables</h2>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Variable names are auto-resolved from the current <code>.env</code> file and Workbench compose references.
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <UiBadge :tone="envExistsTone">
          {{ envExistsLabel }}
        </UiBadge>
        <UiButton
          variant="ghost"
          size="sm"
          :disabled="!isAdmin || loading || saving"
          @click="loadEnv()"
        >
          <span class="inline-flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="loading" />
            Reload
          </span>
        </UiButton>
      </div>
    </div>

    <UiPanel variant="soft" class="grid gap-3 p-4 text-xs text-[color:var(--muted)] sm:grid-cols-2 xl:grid-cols-4">
      <div class="space-y-1">
        <p class="uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Env path</p>
        <p class="font-mono text-[11px] text-[color:var(--text)] break-all">{{ envPathLabel }}</p>
      </div>
      <div class="space-y-1">
        <p class="uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Size</p>
        <p class="text-[color:var(--text)]">{{ envSizeLabel }}</p>
      </div>
      <div class="space-y-1">
        <p class="uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Detected</p>
        <p class="text-[color:var(--text)]">{{ fileVariableCount }} from file</p>
      </div>
      <div class="space-y-1">
        <p class="uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Last updated</p>
        <p class="text-[color:var(--text)]">{{ lastUpdatedLabel }}</p>
      </div>
    </UiPanel>

    <UiState v-if="!isAdmin" tone="warn">
      Read-only access: admin permissions are required to read and update project environment variables.
    </UiState>
    <UiState v-else-if="loading" loading>
      Loading project environment variables...
    </UiState>
    <UiState v-else-if="loadError" tone="error">
      {{ loadError }}
    </UiState>

    <template v-else>
      <UiState v-if="variables.length === 0" tone="neutral">
        No environment variables were detected yet. Add a custom variable below to initialize this file from UI.
      </UiState>

      <div v-else class="grid gap-3 xl:grid-cols-2">
        <UiPanel
          v-for="variable in variables"
          :key="variable.name"
          variant="soft"
          class="space-y-3 p-4"
        >
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="font-mono text-xs text-[color:var(--text)]">{{ variable.name }}</p>
            <div class="flex items-center gap-2">
              <UiBadge :tone="variableSourceTone(variable.source)">
                {{ variableSourceLabel(variable.source) }}
              </UiBadge>
              <UiButton
                v-if="variable.source === 'manual'"
                variant="ghost"
                size="xs"
                :disabled="saving"
                @click="removeManualVariable(variable.name)"
              >
                Remove
              </UiButton>
            </div>
          </div>
          <UiInput
            v-model="variable.value"
            :disabled="saving"
            :placeholder="`Value for ${variable.name}`"
            autocomplete="off"
            spellcheck="false"
          />
        </UiPanel>
      </div>

      <UiPanel variant="raise" class="space-y-3 p-4">
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Add custom variable</p>
        <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-end">
          <label class="grid gap-2">
            <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Variable name</span>
            <UiInput
              v-model="customVariableName"
              :disabled="!canAddCustomVariable"
              placeholder="NEW_VARIABLE"
              autocomplete="off"
              spellcheck="false"
            />
          </label>
          <label class="grid gap-2">
            <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Value</span>
            <UiInput
              v-model="customVariableValue"
              :disabled="!canAddCustomVariable"
              placeholder="value"
              autocomplete="off"
              spellcheck="false"
            />
          </label>
          <UiButton
            variant="ghost"
            size="sm"
            :disabled="!canAddCustomVariable"
            @click="addCustomVariable"
          >
            Add variable
          </UiButton>
        </div>
        <UiInlineFeedback v-if="customVariableError" tone="warn">
          {{ customVariableError }}
        </UiInlineFeedback>
      </UiPanel>

      <UiInlineFeedback v-if="saveError" tone="error">
        {{ saveError }}
      </UiInlineFeedback>
      <UiInlineFeedback v-else-if="saveSuccess" tone="ok">
        {{ saveSuccess }}
      </UiInlineFeedback>

      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-wrap items-center gap-2 text-xs text-[color:var(--muted)]">
          <UiToggle v-model="createBackup" :disabled="saving">
            Create backup before save
          </UiToggle>
          <UiBadge tone="neutral">{{ resolvedOnlyVariableCount }} resolved only</UiBadge>
          <UiBadge tone="neutral">{{ manualVariableCount }} custom</UiBadge>
        </div>
        <UiButton
          variant="primary"
          size="sm"
          :disabled="!canSave"
          @click="saveEnv"
        >
          <span class="inline-flex items-center gap-2">
            <UiInlineSpinner v-if="saving" />
            {{ saving ? 'Saving environment...' : 'Save environment' }}
          </span>
        </UiButton>
      </div>
    </template>
  </UiPanel>
</template>

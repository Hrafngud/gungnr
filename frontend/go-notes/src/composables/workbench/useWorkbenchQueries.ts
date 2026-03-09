import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { workbenchApi } from '@/services/workbench'
import { workbenchQueryKeys } from '@/services/workbenchQueries'

interface UseWorkbenchQueryOptions {
  enabled?: MaybeRefOrGetter<boolean>
}

function normalizeProjectName(projectName: string): string {
  return projectName.trim()
}

function resolveEnabled(
  projectName: MaybeRefOrGetter<string>,
  enabled: MaybeRefOrGetter<boolean> | undefined,
) {
  return computed(() => {
    const normalizedProjectName = normalizeProjectName(toValue(projectName))
    if (!normalizedProjectName) return false
    if (!enabled) return true
    return Boolean(toValue(enabled))
  })
}

function resolveProjectName(projectName: MaybeRefOrGetter<string>) {
  return computed(() => normalizeProjectName(toValue(projectName)))
}

export function useWorkbenchSnapshotQuery(
  projectName: MaybeRefOrGetter<string>,
  options: UseWorkbenchQueryOptions = {},
) {
  const normalizedProjectName = resolveProjectName(projectName)
  const enabled = resolveEnabled(projectName, options.enabled)

  return useQuery({
    queryKey: computed(() => workbenchQueryKeys.snapshot(normalizedProjectName.value)),
    enabled,
    queryFn: async () => {
      const { data } = await workbenchApi.getSnapshot(normalizedProjectName.value)
      return data.stack
    },
  })
}

export function useWorkbenchCatalogQuery(
  projectName: MaybeRefOrGetter<string>,
  options: UseWorkbenchQueryOptions = {},
) {
  const normalizedProjectName = resolveProjectName(projectName)
  const enabled = resolveEnabled(projectName, options.enabled)

  return useQuery({
    queryKey: computed(() => workbenchQueryKeys.catalog(normalizedProjectName.value)),
    enabled,
    queryFn: async () => {
      const { data } = await workbenchApi.getCatalog(normalizedProjectName.value)
      return data.catalog
    },
  })
}

export function useWorkbenchComposeBackupsQuery(
  projectName: MaybeRefOrGetter<string>,
  options: UseWorkbenchQueryOptions = {},
) {
  const normalizedProjectName = resolveProjectName(projectName)
  const enabled = resolveEnabled(projectName, options.enabled)

  return useQuery({
    queryKey: computed(() => workbenchQueryKeys.backups(normalizedProjectName.value)),
    enabled,
    queryFn: async () => {
      const { data } = await workbenchApi.getComposeBackups(normalizedProjectName.value)
      return data.backups
    },
  })
}

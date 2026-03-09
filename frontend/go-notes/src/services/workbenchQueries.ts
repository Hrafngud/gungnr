import type { QueryClient } from '@tanstack/vue-query'

interface WorkbenchReadQuerySelection {
  snapshot?: boolean
  catalog?: boolean
  backups?: boolean
}

function normalizeProjectName(projectName: string): string {
  return projectName.trim()
}

function buildQuerySelection(selection: WorkbenchReadQuerySelection): Required<WorkbenchReadQuerySelection> {
  return {
    snapshot: selection.snapshot ?? true,
    catalog: selection.catalog ?? true,
    backups: selection.backups ?? false,
  }
}

export const workbenchQueryKeys = {
  all: ['workbench'] as const,
  project: (projectName: string) => ['workbench', normalizeProjectName(projectName)] as const,
  snapshot: (projectName: string) =>
    [...workbenchQueryKeys.project(projectName), 'snapshot'] as const,
  catalog: (projectName: string) => [...workbenchQueryKeys.project(projectName), 'catalog'] as const,
  backups: (projectName: string) => [...workbenchQueryKeys.project(projectName), 'backups'] as const,
}

function selectedQueryKeys(
  projectName: string,
  selection: WorkbenchReadQuerySelection = {},
): ReadonlyArray<readonly unknown[]> {
  const normalizedProjectName = normalizeProjectName(projectName)
  if (!normalizedProjectName) return []

  const resolvedSelection = buildQuerySelection(selection)
  const queryKeys: Array<readonly unknown[]> = []
  if (resolvedSelection.snapshot) queryKeys.push(workbenchQueryKeys.snapshot(normalizedProjectName))
  if (resolvedSelection.catalog) queryKeys.push(workbenchQueryKeys.catalog(normalizedProjectName))
  if (resolvedSelection.backups) queryKeys.push(workbenchQueryKeys.backups(normalizedProjectName))
  return queryKeys
}

export async function invalidateWorkbenchReadQueries(
  queryClient: QueryClient,
  projectName: string,
  selection: WorkbenchReadQuerySelection = {},
) {
  const queryKeys = selectedQueryKeys(projectName, selection)
  if (queryKeys.length === 0) return
  await Promise.all(queryKeys.map((queryKey) => queryClient.invalidateQueries({ queryKey })))
}

export async function refetchWorkbenchReadQueries(
  queryClient: QueryClient,
  projectName: string,
  selection: WorkbenchReadQuerySelection = {},
) {
  const queryKeys = selectedQueryKeys(projectName, selection)
  if (queryKeys.length === 0) return
  await Promise.all(
    queryKeys.map((queryKey) =>
      queryClient.refetchQueries({
        queryKey,
        type: 'active',
      }),
    ),
  )
}

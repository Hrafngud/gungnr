import { api } from '@/services/api'
import type {
  WorkbenchComposeApplyRequest,
  WorkbenchComposeApplyResponse,
  WorkbenchComposeBackupsResponse,
  WorkbenchOptionalServiceCatalogResponse,
  WorkbenchComposePreviewRequest,
  WorkbenchComposePreviewResponse,
  WorkbenchComposeRestoreRequest,
  WorkbenchComposeRestoreResponse,
  WorkbenchImportReason,
  WorkbenchImportResponse,
  WorkbenchModuleMutationRequest,
  WorkbenchModuleMutationResponse,
  WorkbenchPortMutationRequest,
  WorkbenchPortMutationResponse,
  WorkbenchPortResolveResponse,
  WorkbenchResourceMutationRequest,
  WorkbenchResourceMutationResponse,
  WorkbenchPortSuggestionRequest,
  WorkbenchPortSuggestionResponse,
  WorkbenchSnapshotResponse,
} from '@/types/workbench'

function workbenchProjectPath(projectName: string): string {
  return `/api/v1/projects/${encodeURIComponent(projectName)}/workbench`
}

export const workbenchApi = {
  getSnapshot: (projectName: string) =>
    api.get<WorkbenchSnapshotResponse>(workbenchProjectPath(projectName)),
  getCatalog: (projectName: string) =>
    api.get<WorkbenchOptionalServiceCatalogResponse>(`${workbenchProjectPath(projectName)}/catalog`),
  importSnapshot: (projectName: string, reason: WorkbenchImportReason) =>
    api.post<WorkbenchImportResponse>(`${workbenchProjectPath(projectName)}/import`, { reason }),
  resolvePorts: (projectName: string) =>
    api.post<WorkbenchPortResolveResponse>(`${workbenchProjectPath(projectName)}/ports/resolve`, {}),
  mutatePort: (projectName: string, payload: WorkbenchPortMutationRequest) =>
    api.post<WorkbenchPortMutationResponse>(`${workbenchProjectPath(projectName)}/ports/mutate`, payload),
  mutateResource: (
    projectName: string,
    serviceName: string,
    payload: WorkbenchResourceMutationRequest,
  ) =>
    api.patch<WorkbenchResourceMutationResponse>(
      `${workbenchProjectPath(projectName)}/services/${encodeURIComponent(serviceName)}/resources`,
      payload,
    ),
  mutateModule: (projectName: string, payload: WorkbenchModuleMutationRequest) =>
    api.post<WorkbenchModuleMutationResponse>(`${workbenchProjectPath(projectName)}/modules`, payload),
  suggestPorts: (projectName: string, payload: WorkbenchPortSuggestionRequest) =>
    api.post<WorkbenchPortSuggestionResponse>(`${workbenchProjectPath(projectName)}/ports/suggest`, payload),
  previewCompose: (projectName: string, payload: WorkbenchComposePreviewRequest) =>
    api.post<WorkbenchComposePreviewResponse>(
      `${workbenchProjectPath(projectName)}/compose/preview`,
      payload,
    ),
  applyCompose: (projectName: string, payload: WorkbenchComposeApplyRequest) =>
    api.post<WorkbenchComposeApplyResponse>(`${workbenchProjectPath(projectName)}/compose/apply`, payload),
  getComposeBackups: (projectName: string) =>
    api.get<WorkbenchComposeBackupsResponse>(`${workbenchProjectPath(projectName)}/compose/backups`),
  restoreCompose: (projectName: string, payload: WorkbenchComposeRestoreRequest) =>
    api.post<WorkbenchComposeRestoreResponse>(`${workbenchProjectPath(projectName)}/compose/restore`, payload),
}

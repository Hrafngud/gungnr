import { api } from '@/services/api'
import type {
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
}

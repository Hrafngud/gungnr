import { api } from '@/services/api'
import type {
  WorkbenchImportReason,
  WorkbenchImportResponse,
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
}

import type { Note, NotePayload } from '@/types/note'
import { api } from './api'

export const notesApi = {
  list: () => api.get<Note[]>('/notes'),
  get: (id: number | string) => api.get<Note>(`/notes/${id}`),
  create: (payload: NotePayload) => api.post<Note>('/notes', payload),
  update: (id: number | string, payload: NotePayload) =>
    api.put<Note>(`/notes/${id}`, payload),
  remove: (id: number | string) => api.delete<void>(`/notes/${id}`),
}

export interface Note {
  id: number
  title: string
  content: string
  tags: string
  createdAt: string
  updatedAt: string
}

export interface NotePayload {
  title: string
  content: string
  tags?: string
}

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { apiErrorMessage, parseApiError } from '@/services/api'
import { notesApi } from '@/services/notes'
import type { Note, NotePayload } from '@/types/note'

export const useNotesStore = defineStore('notes', () => {
  const notes = ref<Note[]>([])
  const listLoading = ref(false)
  const listError = ref<string | null>(null)
  const detailError = ref<string | null>(null)
  const selectedId = ref<number | null>(null)
  const loadingNoteId = ref<number | null>(null)
  const saving = ref(false)
  const deletingId = ref<number | null>(null)

  const selectedNote = computed(() =>
    notes.value.find((note) => note.id === selectedId.value) || null,
  )

  function upsertNote(note: Note) {
    const index = notes.value.findIndex((n) => n.id === note.id)
    if (index !== -1) {
      notes.value.splice(index, 1, note)
    } else {
      notes.value = [note, ...notes.value]
    }
  }

  async function loadNotes(force = false) {
    if (notes.value.length && !force) {
      return
    }

    listLoading.value = true
    listError.value = null
    try {
      const { data } = await notesApi.list()
      notes.value = data
    } catch (err) {
      listError.value = apiErrorMessage(err)
    } finally {
      listLoading.value = false
    }
  }

  function selectNote(id: number | null) {
    selectedId.value = id
    detailError.value = null
  }

  async function fetchNote(id: number) {
    loadingNoteId.value = id
    detailError.value = null
    try {
      const { data } = await notesApi.get(id)
      upsertNote(data)
      selectedId.value = data.id
      return data
    } catch (err) {
      const parsed = parseApiError(err)
      detailError.value = parsed.message
      throw parsed
    } finally {
      loadingNoteId.value = null
    }
  }

  async function createNote(payload: NotePayload) {
    saving.value = true
    detailError.value = null
    try {
      const { data } = await notesApi.create(payload)
      upsertNote(data)
      selectedId.value = data.id
      return data
    } catch (err) {
      throw parseApiError(err)
    } finally {
      saving.value = false
    }
  }

  async function updateNote(id: number, payload: NotePayload) {
    saving.value = true
    try {
      const { data } = await notesApi.update(id, payload)
      upsertNote(data)
      selectedId.value = data.id
      return data
    } catch (err) {
      throw parseApiError(err)
    } finally {
      saving.value = false
    }
  }

  async function deleteNote(id: number) {
    deletingId.value = id
    try {
      await notesApi.remove(id)
      notes.value = notes.value.filter((note) => note.id !== id)
      if (selectedId.value === id) {
        selectedId.value = null
      }
    } catch (err) {
      throw parseApiError(err)
    } finally {
      deletingId.value = null
    }
  }

  return {
    notes,
    listLoading,
    listError,
    detailError,
    selectedId,
    loadingNoteId,
    saving,
    deletingId,
    selectedNote,
    loadNotes,
    selectNote,
    fetchNote,
    createNote,
    updateNote,
    deleteNote,
  }
})

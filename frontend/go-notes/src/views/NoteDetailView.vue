<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { parseApiError } from '@/services/api'
import { useNotesStore } from '@/stores/notes'
import type { NotePayload } from '@/types/note'

const route = useRoute()
const router = useRouter()
const notesStore = useNotesStore()

const form = reactive<NotePayload>({
  title: '',
  content: '',
  tags: '',
})
const formErrors = reactive<Record<string, string>>({})
const feedback = ref<{ type: 'success' | 'error'; text: string } | null>(null)
const fetching = ref(false)

const noteId = computed(() => Number(route.params.id))
const note = computed(() => notesStore.notes.find((n) => n.id === noteId.value) || null)
const isLoading = computed(
  () => fetching.value || notesStore.loadingNoteId === noteId.value,
)

onMounted(async () => {
  if (!notesStore.notes.length && !notesStore.listLoading) {
    await notesStore.loadNotes()
  }
  ensureNote()
})

watch(
  () => route.params.id,
  () => {
    ensureNote()
  },
)

watch(
  note,
  (current) => {
    if (current) {
      syncForm(current)
    }
  },
  { immediate: true },
)

function syncForm(current: { title: string; content: string; tags?: string }) {
  form.title = current.title
  form.content = current.content
  form.tags = current.tags || ''
}

async function ensureNote() {
  feedback.value = null
  clearErrors()

  const id = noteId.value
  if (!Number.isFinite(id) || id <= 0) {
    feedback.value = { type: 'error', text: 'Invalid note id' }
    return
  }

  notesStore.selectNote(id)
  if (note.value) {
    syncForm(note.value)
    return
  }

  fetching.value = true
  try {
    const fetched = await notesStore.fetchNote(id)
    syncForm(fetched)
  } catch (err) {
    feedback.value = { type: 'error', text: parseApiError(err).message }
  } finally {
    fetching.value = false
  }
}

function clearErrors() {
  formErrors.title = ''
  formErrors.content = ''
  formErrors.tags = ''
}

function validate(payload: NotePayload) {
  clearErrors()
  let valid = true
  const title = payload.title.trim()
  if (!title) {
    formErrors.title = 'Title is required'
    valid = false
  } else if (title.length > 255) {
    formErrors.title = 'Title must be at most 255 characters'
    valid = false
  }

  if (payload.content.trim().length > 10000) {
    formErrors.content = 'Content must be at most 10k characters'
    valid = false
  }

  return valid
}

async function saveNote() {
  feedback.value = null
  const id = noteId.value
  if (!Number.isFinite(id) || id <= 0) {
    feedback.value = { type: 'error', text: 'Invalid note id' }
    return
  }

  if (!validate(form)) return

  try {
    const updated = await notesStore.updateNote(id, {
      title: form.title.trim(),
      content: form.content.trim(),
      tags: form.tags?.trim() ?? '',
    })
    syncForm(updated)
    feedback.value = { type: 'success', text: 'Saved changes' }
  } catch (err) {
    const parsed = parseApiError(err)
    feedback.value = { type: 'error', text: parsed.message }
    if (parsed.fields?.title) formErrors.title = parsed.fields.title
    if (parsed.fields?.content) formErrors.content = parsed.fields.content
  }
}

async function deleteNote() {
  const id = noteId.value
  if (!Number.isFinite(id) || id <= 0) return
  const confirmed = window.confirm('Delete this note? This cannot be undone.')
  if (!confirmed) return

  try {
    await notesStore.deleteNote(id)
    feedback.value = { type: 'success', text: 'Note deleted' }
    router.push({ name: 'notes' })
  } catch (err) {
    feedback.value = { type: 'error', text: parseApiError(err).message }
  }
}

function formatDate(value: string) {
  return new Date(value).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <section
    class="rounded-3xl border border-white/10 bg-gradient-to-br from-slate-950 via-slate-900/90 to-slate-900 p-6 shadow-2xl shadow-indigo-900/30"
  >
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.25em] text-indigo-200">
          Note detail
        </p>
        <h1 class="mt-1 text-2xl font-semibold text-white">
          {{ note?.title || `Note #${route.params.id}` }}
        </h1>
        <p class="mt-1 text-sm text-slate-300">
          Edit the title, content, or tags. Changes save back to the API.
        </p>
      </div>
      <button
        type="button"
        class="inline-flex items-center gap-2 rounded-full border border-white/10 px-3 py-2 text-xs font-semibold text-slate-100 transition hover:border-indigo-400 hover:text-white"
        @click="router.push({ name: 'notes' })"
      >
        ← Back to list
      </button>
    </div>

    <div class="mt-5 space-y-4">
      <div
        v-if="isLoading"
        class="space-y-3 rounded-2xl border border-white/10 bg-white/5 p-4"
      >
        <div class="h-6 w-3/5 animate-pulse rounded bg-white/10" />
        <div class="h-28 w-full animate-pulse rounded bg-white/5" />
        <div class="h-5 w-24 animate-pulse rounded bg-white/10" />
      </div>

      <div
        v-else-if="!note"
        class="rounded-2xl border border-amber-400/40 bg-amber-400/10 p-4 text-sm text-amber-50"
      >
        <p class="font-semibold">We couldn’t find that note.</p>
        <p class="mt-1 text-amber-100/90">
          {{ feedback?.text || notesStore.detailError || 'It may have been deleted.' }}
        </p>
        <button
          type="button"
          class="mt-3 inline-flex items-center gap-2 rounded-full bg-white/15 px-3 py-2 text-xs font-semibold text-white transition hover:bg-white/25"
          @click="router.push({ name: 'notes' })"
        >
          Return to notes
        </button>
      </div>

      <div v-else class="grid gap-5 lg:grid-cols-[1.1fr,0.9fr]">
        <form
          class="rounded-2xl border border-white/10 bg-white/5 p-5 shadow-inner shadow-black/20"
          @submit.prevent="saveNote"
        >
          <div class="flex items-center justify-between gap-3">
            <p class="text-xs uppercase tracking-[0.2em] text-indigo-200">
              Edit note
            </p>
            <span class="rounded-full bg-white/10 px-3 py-1 text-[11px] text-slate-200">
              #{{ note.id }}
            </span>
          </div>

          <div class="mt-4 space-y-3">
            <div>
              <label class="text-xs font-semibold uppercase tracking-[0.12em] text-slate-200">
                Title
              </label>
              <input
                v-model="form.title"
                type="text"
                required
                maxlength="255"
                class="mt-1 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-3 py-2 text-sm text-white outline-none ring-indigo-400/70 transition focus:border-indigo-400 focus:ring-2"
              />
              <p v-if="formErrors.title" class="mt-1 text-xs text-amber-200">
                {{ formErrors.title }}
              </p>
            </div>

            <div>
              <label class="text-xs font-semibold uppercase tracking-[0.12em] text-slate-200">
                Content
              </label>
              <textarea
                v-model="form.content"
                rows="8"
                class="mt-1 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-3 py-2 text-sm text-white outline-none ring-indigo-400/70 transition focus:border-indigo-400 focus:ring-2"
              />
              <p v-if="formErrors.content" class="mt-1 text-xs text-amber-200">
                {{ formErrors.content }}
              </p>
            </div>

            <div>
              <label class="text-xs font-semibold uppercase tracking-[0.12em] text-slate-200">
                Tags
              </label>
              <input
                v-model="form.tags"
                type="text"
                class="mt-1 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-3 py-2 text-sm text-white outline-none ring-indigo-400/70 transition focus:border-indigo-400 focus:ring-2"
                placeholder="ideas, backlog, research"
              />
            </div>
          </div>

          <div class="mt-5 flex flex-wrap items-center gap-3">
            <button
              type="submit"
              class="inline-flex items-center gap-2 rounded-full bg-indigo-500 px-4 py-2 text-sm font-semibold text-white shadow-md shadow-indigo-500/30 transition hover:-translate-y-px hover:bg-indigo-400 disabled:cursor-not-allowed disabled:opacity-70"
              :disabled="notesStore.saving"
            >
              {{ notesStore.saving ? 'Saving…' : 'Save changes' }}
            </button>
            <button
              type="button"
              class="inline-flex items-center gap-2 rounded-full bg-red-500/80 px-4 py-2 text-sm font-semibold text-white shadow-md shadow-red-500/30 transition hover:-translate-y-px hover:bg-red-500 disabled:cursor-not-allowed disabled:opacity-70"
              :disabled="notesStore.deletingId === note.id"
              @click="deleteNote"
            >
              {{ notesStore.deletingId === note.id ? 'Deleting…' : 'Delete' }}
            </button>
          </div>

          <p
            v-if="feedback"
            class="mt-3 rounded-xl px-3 py-2 text-xs"
            :class="feedback.type === 'success'
              ? 'border border-emerald-400/40 bg-emerald-400/10 text-emerald-50'
              : 'border border-amber-400/40 bg-amber-400/10 text-amber-50'"
          >
            {{ feedback.text }}
          </p>
        </form>

        <div class="space-y-3 rounded-2xl border border-white/10 bg-slate-950/60 p-5">
          <div class="flex items-center justify-between gap-2">
            <div>
              <p class="text-xs uppercase tracking-[0.2em] text-slate-300">
                Metadata
              </p>
              <p class="text-base font-semibold text-white">Timeline</p>
            </div>
            <span class="rounded-full bg-white/10 px-3 py-1 text-[11px] text-slate-200">
              {{ formatDate(note.updatedAt) }}
            </span>
          </div>
          <dl class="grid grid-cols-2 gap-3 text-sm text-slate-200">
            <div class="rounded-xl border border-white/10 bg-white/5 px-3 py-2">
              <dt class="text-[11px] uppercase tracking-[0.12em] text-slate-400">
                Created
              </dt>
              <dd class="mt-1 font-semibold text-white">
                {{ formatDate(note.createdAt) }}
              </dd>
            </div>
            <div class="rounded-xl border border-white/10 bg-white/5 px-3 py-2">
              <dt class="text-[11px] uppercase tracking-[0.12em] text-slate-400">
                Updated
              </dt>
              <dd class="mt-1 font-semibold text-white">
                {{ formatDate(note.updatedAt) }}
              </dd>
            </div>
          </dl>
          <div class="rounded-xl border border-white/10 bg-white/5 px-3 py-2 text-sm text-slate-200">
            <p class="text-[11px] uppercase tracking-[0.12em] text-slate-400">
              Tags
            </p>
            <div class="mt-2 flex flex-wrap items-center gap-2">
              <span
                v-if="note.tags"
                class="rounded-full bg-indigo-500/20 px-3 py-1 text-xs font-semibold text-indigo-100"
              >
                {{ note.tags }}
              </span>
              <span v-else class="text-xs text-slate-400">No tags</span>
            </div>
          </div>
          <div class="rounded-xl border border-white/10 bg-white/5 px-3 py-2 text-sm text-slate-200">
            <p class="text-[11px] uppercase tracking-[0.12em] text-slate-400">
              Preview
            </p>
            <p class="mt-2 whitespace-pre-line text-slate-100/90">
              {{ note.content || 'No content yet.' }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

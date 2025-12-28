<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { parseApiError } from '@/services/api'
import { useNotesStore } from '@/stores/notes'
import type { NotePayload } from '@/types/note'

const router = useRouter()
const notesStore = useNotesStore()

const createForm = reactive<NotePayload>({
  title: '',
  content: '',
  tags: '',
})
const formErrors = reactive<Record<string, string>>({})
const formMessage = ref<string | null>(null)

const sortedNotes = computed(() =>
  [...notesStore.notes].sort(
    (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
  ),
)

const previewNote = computed(
  () => notesStore.selectedNote || sortedNotes.value[0] || null,
)

onMounted(() => {
  if (!notesStore.notes.length && !notesStore.listLoading) {
    notesStore.loadNotes()
  }
})

watch(
  () => sortedNotes.value,
  (notes) => {
    const first = notes[0]
    if (first && !notesStore.selectedId) {
      notesStore.selectNote(first.id)
    }
  },
  { immediate: true },
)

function formatDate(value: string) {
  return new Date(value).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function validate(payload: NotePayload) {
  formErrors.title = ''
  formErrors.content = ''
  formErrors.tags = ''
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

function resetForm() {
  createForm.title = ''
  createForm.content = ''
  createForm.tags = ''
  formErrors.title = ''
  formErrors.content = ''
  formErrors.tags = ''
}

async function submitNewNote() {
  formMessage.value = null
  if (!validate(createForm)) {
    return
  }

  try {
    const note = await notesStore.createNote({
      title: createForm.title.trim(),
      content: createForm.content.trim(),
      tags: createForm.tags?.trim() ?? '',
    })
    formMessage.value = 'Note created and synced.'
    resetForm()
    router.push({ name: 'note-detail', params: { id: note.id } })
  } catch (err) {
    const parsed = parseApiError(err)
    formMessage.value = parsed.message
    if (parsed.fields?.title) formErrors.title = parsed.fields.title
    if (parsed.fields?.content) formErrors.content = parsed.fields.content
  }
}

function openNote(id: number) {
  notesStore.selectNote(id)
  router.push({ name: 'note-detail', params: { id } })
}

function badgeColors(index: number) {
  const palettes = [
    'from-indigo-500/15 to-cyan-400/15 text-indigo-100 border-indigo-300/40',
    'from-emerald-400/15 to-sky-400/15 text-emerald-100 border-emerald-300/40',
    'from-amber-400/20 to-orange-500/10 text-amber-50 border-amber-200/60',
  ]
  return palettes[index % palettes.length]
}
</script>

<template>
  <section class="grid gap-6 xl:grid-cols-[1.35fr,1fr]">
    <div
      class="rounded-3xl border border-white/10 bg-gradient-to-br from-slate-950 via-slate-900 to-slate-900/70 p-6 shadow-2xl shadow-indigo-900/30"
    >
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p class="text-xs uppercase tracking-[0.25em] text-indigo-200">
            Your notebook
          </p>
          <h1 class="mt-1 text-3xl font-semibold text-white">Notes</h1>
          <p class="mt-2 max-w-2xl text-sm text-slate-300">
            Capture research, todos, and drafts. Click a card to open the detail view and
            edit inline.
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="rounded-full border border-white/10 px-3 py-2 text-xs font-semibold text-slate-100 transition hover:border-indigo-400 hover:text-white"
            :disabled="notesStore.listLoading"
            @click="notesStore.loadNotes(true)"
          >
            {{ notesStore.listLoading ? 'Refreshing…' : 'Refresh list' }}
          </button>
        </div>
      </header>

      <div class="mt-6 space-y-4">
        <div
          v-if="notesStore.listLoading && !sortedNotes.length"
          class="grid gap-3 md:grid-cols-2"
        >
          <div
            v-for="n in 4"
            :key="n"
            class="rounded-2xl border border-white/10 bg-white/5 p-4"
          >
            <div class="h-5 w-2/3 animate-pulse rounded bg-white/10" />
            <div class="mt-2 h-12 w-full animate-pulse rounded bg-white/5" />
            <div class="mt-4 h-4 w-24 animate-pulse rounded bg-white/10" />
          </div>
        </div>

        <div
          v-else-if="notesStore.listError"
          class="flex flex-col gap-3 rounded-2xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-50"
        >
          <div class="flex items-center gap-2 font-semibold">
            <span class="text-base">⚠️</span>
            <p>Unable to load notes</p>
          </div>
          <p class="text-red-100/90">{{ notesStore.listError }}</p>
          <button
            type="button"
            class="self-start rounded-full bg-white/10 px-3 py-2 text-xs font-semibold text-white transition hover:bg-white/20"
            @click="notesStore.loadNotes(true)"
          >
            Try again
          </button>
        </div>

        <ul
          v-else-if="sortedNotes.length"
          class="grid gap-3 md:grid-cols-2"
        >
          <li v-for="(note, index) in sortedNotes" :key="note.id">
            <button
              type="button"
              class="group flex h-full w-full flex-col rounded-2xl border border-white/10 bg-white/5 p-4 text-left transition hover:-translate-y-0.5 hover:border-indigo-400/40 hover:bg-white/10"
              :class="{
                'ring-2 ring-indigo-400/70 ring-offset-2 ring-offset-slate-900':
                  notesStore.selectedId === note.id,
              }"
              @click="notesStore.selectNote(note.id)"
            >
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="text-[11px] uppercase tracking-[0.2em] text-indigo-200">
                    #{{ note.id }}
                  </p>
                  <h3 class="mt-1 text-lg font-semibold text-white line-clamp-1">
                    {{ note.title }}
                  </h3>
                </div>
                <span
                  class="rounded-full border px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.12em]"
                  :class="badgeColors(index)"
                >
                  {{ formatDate(note.updatedAt) }}
                </span>
              </div>
              <p class="mt-3 line-clamp-3 text-sm text-slate-200">
                {{ note.content || 'No content yet.' }}
              </p>
              <div class="mt-3 flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-[0.12em] text-slate-400">
                <span
                  v-if="note.tags"
                  class="rounded-full bg-white/10 px-3 py-1 text-[11px] text-indigo-100"
                >
                  {{ note.tags }}
                </span>
                <span class="rounded-full bg-slate-800/80 px-3 py-1">
                  Updated {{ formatDate(note.updatedAt) }}
                </span>
              </div>
              <div class="mt-4 flex items-center justify-between text-sm">
                <p class="text-slate-300">Created {{ formatDate(note.createdAt) }}</p>
                <button
                  type="button"
                  class="inline-flex items-center gap-1 text-indigo-200 transition hover:text-white"
                  @click.stop="openNote(note.id)"
                >
                  Open
                  <span aria-hidden="true">→</span>
                </button>
              </div>
            </button>
          </li>
        </ul>

        <div
          v-else
          class="rounded-2xl border border-dashed border-white/20 bg-white/5 px-5 py-6 text-sm text-slate-200"
        >
          <p class="text-base font-semibold text-white">No notes yet</p>
          <p class="mt-2 text-slate-300">
            Create your first note using the form on the right. Notes appear here as soon
            as they are saved.
          </p>
          <div class="mt-3 flex flex-wrap items-center gap-2 text-xs text-slate-400">
            <span class="rounded-full bg-white/10 px-3 py-1">GET /api/v1/notes</span>
            <span class="rounded-full bg-white/10 px-3 py-1">POST /api/v1/notes</span>
            <span class="rounded-full bg-white/10 px-3 py-1">PUT /api/v1/notes/:id</span>
          </div>
        </div>
      </div>
    </div>

    <aside class="space-y-4">
      <form
        class="rounded-3xl border border-white/10 bg-gradient-to-br from-indigo-600/30 via-indigo-900/30 to-slate-900/70 p-5 shadow-xl shadow-indigo-900/40"
        @submit.prevent="submitNewNote"
      >
        <p class="text-xs uppercase tracking-[0.25em] text-indigo-100">
          Quick capture
        </p>
        <h2 class="mt-1 text-xl font-semibold text-white">New note</h2>
        <p class="mt-1 text-sm text-indigo-50/90">
          Titles are required; content and tags are optional.
        </p>

        <div class="mt-4 space-y-3">
          <div>
            <label class="text-xs font-semibold uppercase tracking-[0.12em] text-slate-200">
              Title
            </label>
            <input
              v-model="createForm.title"
              type="text"
              required
              maxlength="255"
              class="mt-1 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-3 py-2 text-sm text-white outline-none ring-indigo-400/70 transition focus:border-indigo-400 focus:ring-2"
              placeholder="Plan product launch"
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
              v-model="createForm.content"
              rows="4"
              class="mt-1 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-3 py-2 text-sm text-white outline-none ring-indigo-400/70 transition focus:border-indigo-400 focus:ring-2"
              placeholder="Outline the key milestones, owners, and risks…"
            />
            <p v-if="formErrors.content" class="mt-1 text-xs text-amber-200">
              {{ formErrors.content }}
            </p>
          </div>

          <div>
            <label class="text-xs font-semibold uppercase tracking-[0.12em] text-slate-200">
              Tags (comma separated)
            </label>
            <input
              v-model="createForm.tags"
              type="text"
              class="mt-1 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-3 py-2 text-sm text-white outline-none ring-indigo-400/70 transition focus:border-indigo-400 focus:ring-2"
              placeholder="ideas, backlog, research"
            />
          </div>
        </div>

        <div class="mt-4 flex items-center gap-3">
          <button
            type="submit"
            class="inline-flex items-center gap-2 rounded-full bg-white px-4 py-2 text-sm font-semibold text-slate-900 transition hover:-translate-y-px hover:bg-slate-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white disabled:cursor-not-allowed disabled:opacity-70"
            :disabled="notesStore.saving"
          >
            {{ notesStore.saving ? 'Saving…' : 'Save note' }}
          </button>
          <button
            type="button"
            class="text-xs font-semibold text-slate-200 underline-offset-2 hover:underline"
            @click="resetForm"
          >
            Reset
          </button>
        </div>

        <p
          v-if="formMessage"
          class="mt-3 rounded-xl border border-white/10 bg-white/10 px-3 py-2 text-xs text-white"
        >
          {{ formMessage }}
        </p>
      </form>

      <div
        class="rounded-3xl border border-white/10 bg-gradient-to-br from-slate-900/70 via-slate-900/60 to-slate-950/80 p-5 shadow-xl shadow-slate-900/40"
      >
        <div class="flex items-center justify-between gap-2">
          <div>
            <p class="text-xs uppercase tracking-[0.25em] text-slate-300">
              Peek
            </p>
            <h3 class="text-lg font-semibold text-white">Selected note</h3>
          </div>
          <span class="rounded-full bg-white/10 px-3 py-1 text-xs text-slate-200">
            {{ previewNote ? `#${previewNote.id}` : '—' }}
          </span>
        </div>

        <div v-if="previewNote" class="mt-3 space-y-2 text-sm text-slate-200">
          <p class="text-base font-semibold text-white">{{ previewNote.title }}</p>
          <p class="whitespace-pre-line text-slate-200 line-clamp-6">
            {{ previewNote.content || 'No content yet.' }}
          </p>
          <div class="flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-[0.12em] text-slate-400">
            <span class="rounded-full bg-white/10 px-3 py-1">
              Updated {{ formatDate(previewNote.updatedAt) }}
            </span>
            <span v-if="previewNote.tags" class="rounded-full bg-indigo-500/20 px-3 py-1 text-indigo-100">
              {{ previewNote.tags }}
            </span>
          </div>
          <div class="mt-3">
            <button
              type="button"
              class="inline-flex items-center gap-2 rounded-full bg-indigo-500/90 px-4 py-2 text-xs font-semibold text-white shadow-md shadow-indigo-500/30 transition hover:-translate-y-px hover:bg-indigo-400/90"
              @click="openNote(previewNote.id)"
            >
              Open detail
              <span aria-hidden="true">↗</span>
            </button>
          </div>
        </div>
        <div v-else class="mt-3 text-sm text-slate-300">
          Select a card to preview it here.
        </div>
      </div>
    </aside>
  </section>
</template>

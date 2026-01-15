<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import { usersApi } from '@/services/users'
import { apiErrorMessage } from '@/services/api'
import { usePageLoadingStore } from '@/stores/pageLoading'
import { useToastStore } from '@/stores/toasts'
import { useAuthStore } from '@/stores/auth'
import type { UserRole, UserSummary } from '@/types/users'

const users = ref<UserSummary[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const saving = ref<Record<number, boolean>>({})
const removing = ref<Record<number, boolean>>({})
const adding = ref(false)
const newLogin = ref('')
const pageLoading = usePageLoadingStore()
const toasts = useToastStore()
const auth = useAuthStore()

const roleOptions = [
  { value: 'admin', label: 'Admin' },
  { value: 'user', label: 'User' },
]

const isSaving = (id: number) => Boolean(saving.value[id])
const isRemoving = (id: number) => Boolean(removing.value[id])

const setSaving = (id: number, value: boolean) => {
  if (value) {
    saving.value = { ...saving.value, [id]: true }
    return
  }
  const next = { ...saving.value }
  delete next[id]
  saving.value = next
}

const setRemoving = (id: number, value: boolean) => {
  if (value) {
    removing.value = { ...removing.value, [id]: true }
    return
  }
  const next = { ...removing.value }
  delete next[id]
  removing.value = next
}

const formatLastLogin = (value: string) => {
  if (!value || value.startsWith('0001-')) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Never'
  return date.toLocaleString()
}

const roleTone = (role: UserRole) => {
  if (role === 'superuser') return 'ok'
  if (role === 'admin') return 'warn'
  return 'neutral'
}

const loadUsers = async () => {
  loading.value = true
  error.value = null
  try {
    const { data } = await usersApi.list()
    users.value = data.users ?? []
  } catch (err) {
    error.value = apiErrorMessage(err)
  } finally {
    loading.value = false
  }
}

const upsertUser = (user: UserSummary) => {
  const existingIndex = users.value.findIndex((entry) => entry.id === user.id)
  if (existingIndex === -1) {
    users.value = [...users.value, user]
    return
  }
  const next = [...users.value]
  next[existingIndex] = user
  users.value = next
}

const addUser = async () => {
  const login = newLogin.value.trim().replace(/^@/, '')
  if (!login) {
    toasts.error('Enter a GitHub username.')
    return
  }
  adding.value = true
  try {
    const { data } = await usersApi.create(login)
    upsertUser(data)
    newLogin.value = ''
    toasts.success(`Added @${data.login} to the allowlist.`)
  } catch (err) {
    toasts.error(apiErrorMessage(err))
  } finally {
    adding.value = false
  }
}

const removeUser = async (user: UserSummary) => {
  if (!confirm(`Remove @${user.login} from the allowlist?`)) return
  setRemoving(user.id, true)
  try {
    await usersApi.remove(user.id)
    users.value = users.value.filter((entry) => entry.id !== user.id)
    toasts.success(`Removed @${user.login}.`)
  } catch (err) {
    toasts.error(apiErrorMessage(err))
  } finally {
    setRemoving(user.id, false)
  }
}

const updateRole = async (user: UserSummary, role: UserRole) => {
  if (!canManage.value || user.role === role || user.role === 'superuser') return
  setSaving(user.id, true)
  try {
    const { data } = await usersApi.updateRole(user.id, role)
    users.value = users.value.map((entry) => (entry.id === data.id ? data : entry))
    toasts.success(`Updated @${data.login} to ${data.role}.`)
  } catch (err) {
    toasts.error(apiErrorMessage(err))
  } finally {
    setSaving(user.id, false)
  }
}

const hasUsers = computed(() => users.value.length > 0)
const canManage = computed(() => auth.isAdmin)

onMounted(async () => {
  pageLoading.start('Loading users...')
  await loadUsers()
  pageLoading.stop()
})
</script>

<template>
  <section class="page space-y-10">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Access
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Users and roles
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Manage allowlisted users and assign admin or user roles.
        </p>
      </div>
      <div class="flex flex-wrap gap-3">
        <UiButton variant="ghost" size="sm" :disabled="loading" @click="loadUsers">
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="loading" />
            Refresh list
          </span>
        </UiButton>
      </div>
    </div>

    <UiState v-if="error" tone="error">
      {{ error }}
    </UiState>

    <hr />

    <UiPanel as="article" class="space-y-4 p-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Allowlist
          </p>
          <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
            Approved operators
          </h2>
        </div>
        <div class="flex flex-col items-end gap-3">
          <UiBadge tone="neutral">
            {{ users.length }} total
          </UiBadge>
          <div class="flex flex-wrap items-center justify-end gap-2">
            <UiInput
              v-model="newLogin"
              class="min-w-[200px] max-w-full flex-1"
              placeholder="GitHub username"
              :disabled="!canManage || adding"
              @keyup.enter="addUser"
            />
            <UiButton size="sm" :disabled="!canManage || adding" @click="addUser">
              <span class="flex items-center gap-2">
                <UiInlineSpinner v-if="adding" />
                Add user
              </span>
            </UiButton>
          </div>
          <p v-if="!canManage" class="text-xs text-[color:var(--muted)]">
            View only. Ask an admin to manage allowlist access.
          </p>
        </div>
      </div>

      <div class="grid min-w-0 gap-3 text-[11px] uppercase tracking-[0.3em] text-[color:var(--muted-2)] sm:grid-cols-[minmax(0,1.4fr)_minmax(0,0.8fr)_minmax(0,1fr)_auto_auto]">
        <span>User</span>
        <span>Role</span>
        <span>Last login</span>
        <span class="text-right">Status</span>
        <span class="text-right">Actions</span>
      </div>

      <div v-if="loading && !hasUsers" class="space-y-3">
        <UiSkeleton variant="block" class="h-14" />
        <UiSkeleton variant="block" class="h-14" />
        <UiSkeleton variant="block" class="h-14" />
      </div>

      <UiState v-else-if="!hasUsers" tone="neutral">
        No allowlisted users yet.
      </UiState>

      <div v-else class="space-y-3">
        <UiListRow
          v-for="entry in users"
          :key="entry.id"
          class="grid min-w-0 items-center gap-3 sm:grid-cols-[minmax(0,1.4fr)_minmax(0,0.8fr)_minmax(0,1fr)_auto_auto]"
        >
          <div class="min-w-0 space-y-1">
            <p class="truncate text-sm font-semibold text-[color:var(--text)]">@{{ entry.login }}</p>
            <p class="text-xs text-[color:var(--muted-2)]">ID {{ entry.id }}</p>
          </div>

          <div>
            <UiBadge v-if="entry.role === 'superuser'" :tone="roleTone(entry.role)">
              SuperUser
            </UiBadge>
            <UiSelect
              v-else
              class="min-w-[140px] max-w-full"
              :model-value="entry.role"
              :options="roleOptions"
              :disabled="!canManage || isSaving(entry.id)"
              @update:modelValue="(value) => updateRole(entry, value as UserRole)"
            />
          </div>

          <p class="text-xs text-[color:var(--muted)]">
            {{ formatLastLogin(entry.lastLoginAt) }}
          </p>

          <div class="flex items-center justify-end gap-2 text-xs text-[color:var(--muted)]">
            <UiInlineSpinner v-if="isSaving(entry.id) || isRemoving(entry.id)" />
            <span v-if="isSaving(entry.id)">Updating</span>
            <span v-else-if="isRemoving(entry.id)">Removing</span>
            <span v-else-if="entry.role === 'superuser'">Managed by env</span>
            <span v-else-if="canManage">Editable</span>
            <span v-else>View only</span>
          </div>

          <div class="flex items-center justify-end">
            <UiButton
              v-if="canManage && entry.role !== 'superuser'"
              variant="ghost"
              size="sm"
              :disabled="isRemoving(entry.id)"
              @click="removeUser(entry)"
            >
              <span class="flex items-center gap-2">
                <NavIcon name="trash" class="h-3.5 w-3.5" />
                Remove
              </span>
            </UiButton>
          </div>
        </UiListRow>
      </div>
    </UiPanel>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Breadcrumb from 'primevue/breadcrumb'
import Button from 'primevue/button'
import Menu from 'primevue/menu'
import Message from 'primevue/message'
import Skeleton from 'primevue/skeleton'
import Tag from 'primevue/tag'
import type { MenuItem } from 'primevue/menuitem'
import { useConfirm } from 'primevue/useconfirm'
import type { DirectoryNode, LibraryTree, VideoSummary } from '@/types'
import { api, errorMessage, type ShareTarget } from '@/services/api'
import { useDelayedFlag } from '@/composables/useDelayedFlag'
import { findNode, pathTo } from '@/utils/tree'
import { formatDuration } from '@/utils/format'
import TextPromptDialog from '@/components/TextPromptDialog.vue'
import MoveDialog, { type MoveTarget } from '@/components/MoveDialog.vue'
import DeleteDirectoryDialog from '@/components/DeleteDirectoryDialog.vue'
import ShareDialog from '@/components/ShareDialog.vue'
import AccessDialog from '@/components/AccessDialog.vue'

const route = useRoute()
const router = useRouter()
const confirm = useConfirm()

const creatorId = computed(() => String(route.params.creatorId))
const tree = ref<LibraryTree | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
// Only the first load shows a skeleton; reloads after an edit keep the current list on screen.
const slow = useDelayedFlag(() => loading.value && !tree.value)

const currentId = computed(() => (typeof route.query.dir === 'string' ? route.query.dir : tree.value?.root.id) ?? null)
const current = computed<DirectoryNode | null>(() =>
  tree.value && currentId.value ? findNode(tree.value.root, currentId.value) : null,
)
const crumbs = computed(() => (tree.value && currentId.value ? pathTo(tree.value.root, currentId.value) : []))
const canManage = computed(() => tree.value?.canManage ?? false)

const crumbItems = computed<MenuItem[]>(() =>
  crumbs.value.map((c, i) => ({
    label: dirName(c),
    command: i < crumbs.value.length - 1 ? () => openDir(c.id) : undefined,
    class: i === crumbs.value.length - 1 ? 'crumb-current' : undefined,
  })),
)

async function load() {
  loading.value = true
  error.value = null
  try {
    tree.value = await api.library.tree(creatorId.value)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(creatorId, load)

function openDir(id: string) {
  void router.push({ name: 'collection', params: { creatorId: creatorId.value }, query: { dir: id } })
}

function openVideo(v: VideoSummary) {
  void router.push({ name: 'video', params: { videoId: v.id } })
}

function statusTag(v: VideoSummary): { label: string; severity: 'danger' | 'warn' } | null {
  if (v.status === 'ready') return null
  if (v.status === 'failed') return { label: 'Failed', severity: 'danger' }
  return { label: v.status === 'uploading' ? 'Uploading' : 'Processing', severity: 'warn' }
}

function plural(n: number, word: string) {
  return `${n} ${word}${n === 1 ? '' : 's'}`
}

/* ---------- creator actions ---------- */

interface Selected {
  kind: 'directory' | 'video'
  id: string
  name: string
  directoryId: string
}

const selected = ref<Selected | null>(null)
const dialog = ref<'share' | 'access' | 'rename' | 'move' | 'deleteDir' | 'newFolder' | null>(null)
const busy = ref(false)
const menu = ref<InstanceType<typeof Menu> | null>(null)

const menuItems = computed<MenuItem[]>(() => {
  const s = selected.value
  if (!s) return []
  const items: MenuItem[] = [
    { label: 'Share (QR code)', icon: 'pi pi-qrcode', command: () => (dialog.value = 'share') },
    { label: 'View / edit access', icon: 'pi pi-users', command: () => (dialog.value = 'access') },
  ]
  // The root folder is the collection itself: it cannot be renamed, moved or deleted.
  if (s.kind === 'directory' && s.id === tree.value?.root.id) return items
  items.push(
    { label: 'Rename', icon: 'pi pi-pencil', command: () => (dialog.value = 'rename') },
    { label: 'Move', icon: 'pi pi-arrows-alt', command: () => (dialog.value = 'move') },
    { separator: true },
    { label: 'Delete', icon: 'pi pi-trash', class: 'menu-danger', command: () => askDelete() },
  )
  return items
})

function openMenu(event: Event, kind: 'directory' | 'video', id: string, name: string, directoryId: string) {
  selected.value = { kind, id, name, directoryId }
  menu.value?.toggle(event)
}

function askDelete() {
  const s = selected.value
  if (!s) return
  if (s.kind === 'directory') {
    dialog.value = 'deleteDir'
    return
  }
  confirm.require({
    header: 'Delete video',
    message: `Delete “${s.name}”? It is removed from Vimeo as well. This cannot be undone.`,
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', text: true },
    acceptProps: { label: 'Delete', severity: 'danger' },
    accept: () => void run(() => api.videos.remove(s.id)),
  })
}

const shareTarget = computed<ShareTarget | null>(() => {
  const s = selected.value
  if (!s) return null
  return s.kind === 'directory' ? { directoryId: s.id } : { videoId: s.id }
})

const moveTarget = computed<MoveTarget | null>(() => {
  const s = selected.value
  return s ? { kind: s.kind, id: s.id, name: s.name, currentDirectoryId: s.directoryId } : null
})

const selectedNode = computed(() =>
  selected.value?.kind === 'directory' && tree.value ? findNode(tree.value.root, selected.value.id) : null,
)

async function run(action: () => Promise<unknown>) {
  busy.value = true
  error.value = null
  try {
    await action()
    dialog.value = null
    await load()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}

function rename(name: string) {
  const s = selected.value
  if (!s) return
  void run(() => (s.kind === 'directory' ? api.directories.rename(s.id, name) : api.videos.rename(s.id, name)))
}

function createFolder(name: string) {
  const parent = currentId.value
  if (!parent) return
  void run(() => api.directories.create(name, parent))
}

function afterChange() {
  dialog.value = null
  void load()
}

function dirName(node: DirectoryNode) {
  return node.parentId === null ? (tree.value?.creator.name ?? node.name) : node.name
}
</script>

<template>
  <Message v-if="error" severity="error" class="mb">{{ error }}</Message>

  <ul v-if="slow" class="list">
    <li v-for="i in 4" :key="i" class="row">
      <Skeleton shape="square" size="40px" border-radius="10px" />
      <div class="grow"><Skeleton width="55%" class="mb" /><Skeleton width="30%" height="0.75rem" /></div>
    </li>
  </ul>

  <template v-if="tree && current">
    <Breadcrumb v-if="crumbs.length > 1" :model="crumbItems" aria-label="Folder path" />

    <div class="page-head">
      <h2>{{ dirName(current) }}</h2>
      <template v-if="canManage">
        <Button
          label="New folder"
          icon="pi pi-folder-plus"
          severity="secondary"
          outlined
          size="small"
          @click="dialog = 'newFolder'"
        />
        <Button
          icon="pi pi-ellipsis-v"
          severity="secondary"
          text
          rounded
          aria-label="Folder options"
          @click="openMenu($event, 'directory', current.id, dirName(current), current.parentId ?? current.id)"
        />
      </template>
    </div>
    <p v-if="canManage && crumbs.length <= 1" class="hint">You manage this collection.</p>

    <div v-if="current.children.length === 0 && current.videos.length === 0" class="empty">
      <i class="pi pi-folder-open" aria-hidden="true" />
      <p>This folder is empty.</p>
      <Button
        v-if="canManage"
        as="router-link"
        :to="{ name: 'upload', query: { dir: current.id } }"
        label="Upload a video"
        icon="pi pi-upload"
      />
    </div>

    <ul v-else class="list">
      <li
        v-for="d in current.children"
        :key="d.id"
        class="row clickable"
        role="button"
        tabindex="0"
        @click="openDir(d.id)"
        @keydown.enter="openDir(d.id)"
      >
        <span class="lead"><i class="pi pi-folder" aria-hidden="true" /></span>
        <div class="grow">
          <div class="title">{{ d.name }}</div>
          <div class="sub">{{ plural(d.children.length, 'folder') }} · {{ plural(d.videos.length, 'video') }}</div>
        </div>
        <Button
          v-if="canManage"
          icon="pi pi-ellipsis-v"
          severity="secondary"
          text
          rounded
          aria-label="Options"
          @click.stop="openMenu($event, 'directory', d.id, d.name, current.id)"
        />
        <i v-else class="pi pi-chevron-right trail" aria-hidden="true" />
      </li>

      <li
        v-for="v in current.videos"
        :key="v.id"
        class="row clickable"
        role="button"
        tabindex="0"
        @click="openVideo(v)"
        @keydown.enter="openVideo(v)"
      >
        <span class="lead"><i class="pi pi-video" aria-hidden="true" /></span>
        <div class="grow">
          <div class="title">{{ v.title }}</div>
          <div class="sub">
            <Tag v-if="statusTag(v)" :value="statusTag(v)?.label" :severity="statusTag(v)?.severity" />
            <template v-else>{{ formatDuration(v.durationSec) }}</template>
          </div>
        </div>
        <Button
          v-if="canManage"
          icon="pi pi-ellipsis-v"
          severity="secondary"
          text
          rounded
          aria-label="Options"
          @click.stop="openMenu($event, 'video', v.id, v.title, v.directoryId)"
        />
      </li>
    </ul>

    <!-- creator menus and dialogs -->
    <Menu ref="menu" :model="menuItems" popup />
    <ShareDialog :open="dialog === 'share'" :target="shareTarget" :name="selected?.name ?? ''" @close="dialog = null" />
    <AccessDialog :open="dialog === 'access'" :target="shareTarget" :name="selected?.name ?? ''" @close="dialog = null" />
    <TextPromptDialog
      :open="dialog === 'rename'"
      title="Rename"
      label="New name"
      :initial="selected?.name"
      confirm-label="Rename"
      :busy="busy"
      @close="dialog = null"
      @submit="rename"
    />
    <TextPromptDialog
      :open="dialog === 'newFolder'"
      title="New folder"
      label="Folder name"
      confirm-label="Create"
      :busy="busy"
      @close="dialog = null"
      @submit="createFolder"
    />
    <MoveDialog
      :open="dialog === 'move'"
      :target="moveTarget"
      :root="tree.root"
      :creator-name="tree.creator.name"
      @close="dialog = null"
      @moved="afterChange"
    />
    <DeleteDirectoryDialog
      :open="dialog === 'deleteDir'"
      :node="selectedNode"
      :root="tree.root"
      :creator-name="tree.creator.name"
      @close="dialog = null"
      @deleted="afterChange"
    />
  </template>
</template>

<style scoped>
:deep(.p-breadcrumb-list) {
  flex-wrap: wrap;
}

:deep(.p-breadcrumb-item-label) {
  white-space: nowrap;
}

:deep(.crumb-current .p-breadcrumb-item-label) {
  color: var(--p-text-color);
  font-weight: 500;
}
</style>

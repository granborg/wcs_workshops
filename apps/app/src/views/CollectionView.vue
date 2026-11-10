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
import type { DirectoryNode, LibraryTree, VideoStatus, VideoSummary } from '@/types'
import { api, errorMessage, type ShareTarget } from '@/services/api'
import { useDelayedFlag } from '@/composables/useDelayedFlag'
import { findNode, flatten, pathTo } from '@/utils/tree'
import { formatDuration } from '@/utils/format'
import TextPromptDialog from '@/components/TextPromptDialog.vue'
import MoveDialog, { type MoveTarget } from '@/components/MoveDialog.vue'
import DeleteDirectoryDialog from '@/components/DeleteDirectoryDialog.vue'
import ShareDialog from '@/components/ShareDialog.vue'
import AccessDialog from '@/components/AccessDialog.vue'
import AdminsDialog from '@/components/AdminsDialog.vue'
import UploadLinkDialog from '@/components/UploadLinkDialog.vue'

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
// The owner manages everything, folders and admins included. An admin manages the videos, codes and access of
// their folders and of every folder under them.
const canManage = computed(() => tree.value?.canManage ?? false)
const adminIds = computed(() => new Set(tree.value?.adminDirectoryIds ?? []))
function managesDir(id: string) {
  if (canManage.value) return true
  if (!tree.value || adminIds.value.size === 0) return false
  return pathTo(tree.value.root, id).some((n) => adminIds.value.has(n.id))
}
const canManageHere = computed(() => !!current.value && managesDir(current.value.id))
// Folders a video can move to: all for the owner, only managed folders for an admin.
const managedFolderIds = computed(() =>
  tree.value ? new Set(flatten(tree.value.root).filter((d) => managesDir(d.id)).map((d) => d.id)) : new Set<string>(),
)

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

function statusTag(v: VideoSummary): { label: string; severity: 'danger' | 'warn' | 'info' } | null {
  if (v.status === 'ready') return null
  if (v.status === 'failed') return { label: 'Failed', severity: 'danger' }
  if (v.status === 'placeholder') return { label: 'Not uploaded yet', severity: 'info' }
  return { label: v.status === 'uploading' ? 'Uploading' : 'Processing', severity: 'warn' }
}

/* ---------- creator actions ---------- */

interface Selected {
  kind: 'directory' | 'video'
  id: string
  name: string
  directoryId: string
  status?: VideoStatus
}

// A video with one of these statuses has no file yet, so an upload link can supply it.
const NEEDS_FILE: VideoStatus[] = ['placeholder', 'uploading', 'failed']

const selected = ref<Selected | null>(null)
const dialog = ref<'share' | 'access' | 'rename' | 'move' | 'deleteDir' | 'newFolder' | 'admins' | 'uploadLink' | null>(null)
const busy = ref(false)
const menu = ref<InstanceType<typeof Menu> | null>(null)

const menuItems = computed<MenuItem[]>(() => {
  const s = selected.value
  if (!s) return []
  const items: MenuItem[] = [
    { label: 'Share (QR code)', icon: 'pi pi-qrcode', command: () => (dialog.value = 'share') },
    { label: 'View / edit access', icon: 'pi pi-users', command: () => (dialog.value = 'access') },
  ]
  const change: MenuItem[] = [
    { label: 'Rename', icon: 'pi pi-pencil', command: () => (dialog.value = 'rename') },
    { label: 'Move', icon: 'pi pi-arrows-alt', command: () => (dialog.value = 'move') },
    { separator: true },
    { label: 'Delete', icon: 'pi pi-trash', class: 'menu-danger', command: () => askDelete() },
  ]
  if (s.kind === 'video') {
    if (s.status && NEEDS_FILE.includes(s.status)) {
      items.push({ label: 'Upload link', icon: 'pi pi-cloud-upload', command: () => (dialog.value = 'uploadLink') })
    }
    return [...items, ...change]
  }
  // Folder changes and admins are for the owner only.
  if (!canManage.value) return items
  items.unshift({ label: 'New folder', icon: 'pi pi-folder-plus', command: () => (dialog.value = 'newFolder') }, { separator: true })
  items.push({ label: 'Admins', icon: 'pi pi-user-edit', command: () => (dialog.value = 'admins') })
  // The root folder is the collection itself: it cannot be renamed, moved or deleted.
  if (s.id === tree.value?.root.id) return items
  return [...items, ...change]
})

function openMenu(
  event: Event,
  kind: 'directory' | 'video',
  id: string,
  name: string,
  directoryId: string,
  status?: VideoStatus,
) {
  selected.value = { kind, id, name, directoryId, status }
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

// The new folder goes into the folder whose menu was used.
function createFolder(name: string) {
  const parent = selected.value?.kind === 'directory' ? selected.value.id : currentId.value
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
    <div class="card head">
      <Breadcrumb v-if="crumbs.length > 1" :model="crumbItems" aria-label="Folder path" />

      <div class="page-head">
        <h2>{{ dirName(current) }}</h2>
        <Button
          v-if="canManageHere"
          icon="pi pi-ellipsis-v"
          severity="secondary"
          text
          rounded
          aria-label="Folder options"
          @click="openMenu($event, 'directory', current.id, dirName(current), current.parentId ?? current.id)"
        />
      </div>
      <p v-if="canManage && crumbs.length <= 1" class="hint">You manage this collection.</p>
      <p v-else-if="canManageHere && !canManage" class="hint">You are an admin of this folder.</p>
    </div>

    <div v-if="current.children.length === 0 && current.videos.length === 0" class="empty">
      <i class="pi pi-folder-open" aria-hidden="true" />
      <p>This folder is empty.</p>
      <Button
        v-if="canManageHere"
        as="router-link"
        :to="{ name: 'upload', query: { creator: creatorId, dir: current.id } }"
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
        </div>
        <Button
          v-if="managesDir(d.id)"
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
        <span class="lead" :class="{ thumb: v.thumbnailUrl }">
          <img v-if="v.thumbnailUrl" :src="v.thumbnailUrl" alt="" loading="lazy" />
          <i v-else class="pi pi-video" aria-hidden="true" />
        </span>
        <div class="grow">
          <div class="title">{{ v.title }}</div>
          <div class="sub">
            <Tag v-if="statusTag(v)" :value="statusTag(v)?.label" :severity="statusTag(v)?.severity" />
            <template v-else>{{ formatDuration(v.durationSec) }}</template>
          </div>
        </div>
        <Button
          v-if="canManageHere"
          icon="pi pi-ellipsis-v"
          severity="secondary"
          text
          rounded
          aria-label="Options"
          @click.stop="openMenu($event, 'video', v.id, v.title, v.directoryId, v.status)"
        />
      </li>
    </ul>

    <!-- creator menus and dialogs -->
    <Menu ref="menu" :model="menuItems" popup />
    <ShareDialog :open="dialog === 'share'" :target="shareTarget" :name="selected?.name ?? ''" @close="dialog = null" />
    <AccessDialog :open="dialog === 'access'" :target="shareTarget" :name="selected?.name ?? ''" @close="dialog = null" />
    <AdminsDialog
      :open="dialog === 'admins'"
      :directory-id="selected?.kind === 'directory' ? selected.id : null"
      :name="selected?.name ?? ''"
      :is-root="selected?.id === tree.root.id"
      @close="dialog = null"
    />
    <UploadLinkDialog
      :open="dialog === 'uploadLink'"
      :video-id="selected?.kind === 'video' ? selected.id : null"
      :name="selected?.name ?? ''"
      @close="dialog = null"
    />
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
      :allowed="canManage ? null : managedFolderIds"
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
/* The breadcrumb, folder name, and hint share one card. */
.head .p-breadcrumb {
  padding: 0 0 8px;
  background: none;
}

.head .page-head,
.head .hint {
  margin-bottom: 0;
}

.head .hint {
  margin-top: 4px;
}
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

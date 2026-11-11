<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Breadcrumb from 'primevue/breadcrumb'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable, { type DataTableRowClickEvent } from 'primevue/datatable'
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
import { formatDate, formatDuration } from '@/utils/format'
import TextPromptDialog from '@/components/TextPromptDialog.vue'
import MoveDialog, { type MoveTarget } from '@/components/MoveDialog.vue'
import DeleteDirectoryDialog from '@/components/DeleteDirectoryDialog.vue'
import ShareDialog from '@/components/ShareDialog.vue'
import AccessDialog from '@/components/AccessDialog.vue'
import AdminsDialog from '@/components/AdminsDialog.vue'
import UploadLinkDialog from '@/components/UploadLinkDialog.vue'
import VideoDateDialog from '@/components/VideoDateDialog.vue'
import DownloadDialog from '@/components/DownloadDialog.vue'

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

// While a video is processing on Vimeo, load again now and then. The server asks Vimeo for the status of
// processing videos on each load, so the list changes to ready without the user opening the video.
let poll: ReturnType<typeof setTimeout> | null = null

function hasProcessing(node: DirectoryNode): boolean {
  return node.videos.some((v) => v.status === 'processing') || node.children.some(hasProcessing)
}

async function load() {
  if (poll) clearTimeout(poll)
  poll = null
  loading.value = true
  error.value = null
  try {
    tree.value = await api.library.tree(creatorId.value)
    if (hasProcessing(tree.value.root)) poll = setTimeout(load, 15_000)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(creatorId, load)
onBeforeUnmount(() => {
  if (poll) clearTimeout(poll)
})

function openDir(id: string) {
  void router.push({ name: 'collection', params: { creatorId: creatorId.value }, query: { dir: id } })
}

function openVideo(v: VideoSummary) {
  void router.push({ name: 'video', params: { videoId: v.id } })
}

function uploadTo(directoryId: string) {
  void router.push({ name: 'upload', query: { creator: creatorId.value, dir: directoryId } })
}

/* ---------- video sort: by title or by date, kept on this device ---------- */

type SortField = 'title' | 'videoDate'
const SORT_KEY = 'wcs.videoSort'

function loadSort(): { field: SortField; order: 1 | -1 } {
  try {
    const saved = JSON.parse(localStorage.getItem(SORT_KEY) ?? 'null') as { field?: unknown; order?: unknown } | null
    if ((saved?.field === 'title' || saved?.field === 'videoDate') && (saved.order === 1 || saved.order === -1)) {
      return { field: saved.field, order: saved.order }
    }
  } catch {
    // No storage on this device, or a bad value: use the default.
  }
  return { field: 'title', order: 1 }
}

const savedSort = loadSort()
const sortField = ref<SortField>(savedSort.field)
const sortOrder = ref<1 | -1>(savedSort.order)
watch([sortField, sortOrder], ([field, order]) => {
  try {
    localStorage.setItem(SORT_KEY, JSON.stringify({ field, order }))
  } catch {
    // The sort still works; it is not kept.
  }
})

function onSortField(v: unknown) {
  if (v === 'title' || v === 'videoDate') sortField.value = v
}

function onSortOrder(v: number | undefined | null) {
  if (v === 1 || v === -1) sortOrder.value = v
}

function onRowClick(e: DataTableRowClickEvent) {
  openVideo(e.data as VideoSummary)
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
  videoDate?: string | null
}

// A video with one of these statuses has no file yet, so an upload link can supply it.
const NEEDS_FILE: VideoStatus[] = ['placeholder', 'uploading', 'failed']

const selected = ref<Selected | null>(null)
const dialog = ref<
  | 'share'
  | 'access'
  | 'rename'
  | 'date'
  | 'download'
  | 'move'
  | 'deleteDir'
  | 'newFolder'
  | 'admins'
  | 'uploadLink'
  | null
>(null)
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
    // Vimeo has download files only when it has finished processing. There is no download for a folder:
    // Vimeo has no bulk download.
    if (s.status === 'ready') {
      items.push({ label: 'Download', icon: 'pi pi-download', command: () => (dialog.value = 'download') })
    }
    const setDate: MenuItem = { label: 'Set date', icon: 'pi pi-calendar', command: () => (dialog.value = 'date') }
    return [...items, change[0]!, setDate, ...change.slice(1)]
  }
  // The owner and the admins of a folder can add videos to it.
  const upload: MenuItem = { label: 'Upload video', icon: 'pi pi-upload', command: () => uploadTo(s.id) }
  // Folder changes and admins are for the owner only.
  if (!canManage.value) return [upload, { separator: true }, ...items]
  items.unshift(
    { label: 'New folder', icon: 'pi pi-folder-plus', command: () => (dialog.value = 'newFolder') },
    upload,
    { separator: true },
  )
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
  videoDate?: string | null,
) {
  selected.value = { kind, id, name, directoryId, status, videoDate }
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

function setDate(value: string | null) {
  const s = selected.value
  if (!s || s.kind !== 'video') return
  void run(() => api.videos.setDate(s.id, value))
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

    <template v-else>
      <ul v-if="current.children.length > 0" class="list">
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
      </ul>

      <!-- Select a column header to sort by title or by date. Select it again to reverse the order. -->
      <div v-if="current.videos.length > 0" class="videos">
        <DataTable
          :value="current.videos"
          data-key="id"
          :sort-field="sortField"
          :sort-order="sortOrder"
          selection-mode="single"
          aria-label="Videos"
          @update:sort-field="onSortField"
          @update:sort-order="onSortOrder"
          @row-click="onRowClick"
        >
          <Column field="title" header="Title" sortable>
            <template #body="{ data }: { data: VideoSummary }">
              <div class="video-cell">
                <img v-if="data.thumbnailUrl" class="thumb" :src="data.thumbnailUrl" alt="" loading="lazy" />
                <span v-else class="lead"><i class="pi pi-video" aria-hidden="true" /></span>
                <div class="grow">
                  <div class="title">{{ data.title }}</div>
                  <div class="sub">
                    <Tag v-if="statusTag(data)" :value="statusTag(data)?.label" :severity="statusTag(data)?.severity" />
                    <template v-else>{{ formatDuration(data.durationSec) }}</template>
                  </div>
                </div>
              </div>
            </template>
          </Column>
          <Column field="videoDate" header="Date" sortable class="date-col">
            <template #body="{ data }: { data: VideoSummary }">{{ formatDate(data.videoDate) }}</template>
          </Column>
          <Column v-if="canManageHere" class="action-col">
            <template #body="{ data }: { data: VideoSummary }">
              <Button
                icon="pi pi-ellipsis-v"
                severity="secondary"
                text
                rounded
                aria-label="Options"
                @click.stop="openMenu($event, 'video', data.id, data.title, data.directoryId, data.status, data.videoDate)"
              />
            </template>
          </Column>
        </DataTable>
      </div>
    </template>

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
    <DownloadDialog
      :open="dialog === 'download'"
      :video-id="selected?.kind === 'video' ? selected.id : null"
      :name="selected?.name ?? ''"
      @close="dialog = null"
    />
    <VideoDateDialog
      :open="dialog === 'date'"
      :name="selected?.name ?? ''"
      :initial="selected?.videoDate ?? null"
      :busy="busy"
      @close="dialog = null"
      @submit="setDate"
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

/* The video table sits on a card, like the folder list. */
.videos {
  margin-bottom: 16px;
  background: var(--p-content-background);
  border: 1px solid var(--p-content-border-color);
  border-radius: var(--radius);
  box-shadow: var(--card-shadow);
  overflow: hidden;
}

.video-cell {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

/* A 16:9 picture from the video. A portrait video is cut to fit; the video page shows its real shape. */
.video-cell .thumb {
  flex: none;
  width: 72px;
  height: 40px;
  object-fit: cover;
  border-radius: 8px;
  background: var(--p-content-border-color);
}

/* The same text layout as a .row in the folder list. */
.video-cell .grow {
  flex: 1;
  min-width: 0;
}

.video-cell .title {
  font-weight: 500;
  overflow-wrap: anywhere;
}

.video-cell .sub {
  margin-top: 2px;
  font-size: 0.8125rem;
  color: var(--p-text-muted-color);
}

/* The date is short: keep it on one line and let the title take the rest of the width. */
.videos :deep(.date-col) {
  width: 1%;
  white-space: nowrap;
  color: var(--p-text-muted-color);
  font-size: 0.875rem;
}

.videos :deep(th.date-col) {
  color: inherit;
  font-size: inherit;
}

.videos :deep(.action-col) {
  width: 1%;
  padding-left: 0;
  padding-right: 8px;
}

/* Narrow screens: less cell padding so the title has room. */
@media (max-width: 480px) {
  .videos :deep(.p-datatable-tbody > tr > td),
  .videos :deep(.p-datatable-thead > tr > th) {
    padding-left: 10px;
    padding-right: 10px;
  }

  .video-cell .lead {
    display: none;
  }

  .video-cell .thumb {
    width: 56px;
    height: 32px;
  }
}
</style>

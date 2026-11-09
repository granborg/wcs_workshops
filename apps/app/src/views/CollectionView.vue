<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { DirectoryNode, LibraryTree, VideoSummary } from '@/types'
import { api, errorMessage, type ShareTarget } from '@/services/api'
import { findNode, pathTo } from '@/utils/tree'
import { formatDuration } from '@/utils/format'
import Icon from '@/components/Icon.vue'
import ActionSheet, { type SheetAction } from '@/components/ActionSheet.vue'
import TextPromptDialog from '@/components/TextPromptDialog.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import MoveDialog, { type MoveTarget } from '@/components/MoveDialog.vue'
import DeleteDirectoryDialog from '@/components/DeleteDirectoryDialog.vue'
import ShareDialog from '@/components/ShareDialog.vue'
import AccessDialog from '@/components/AccessDialog.vue'

const route = useRoute()
const router = useRouter()

const creatorId = computed(() => String(route.params.creatorId))
const tree = ref<LibraryTree | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const currentId = computed(() => (typeof route.query.dir === 'string' ? route.query.dir : tree.value?.root.id) ?? null)
const current = computed<DirectoryNode | null>(() =>
  tree.value && currentId.value ? findNode(tree.value.root, currentId.value) : null,
)
const crumbs = computed(() => (tree.value && currentId.value ? pathTo(tree.value.root, currentId.value) : []))
const canManage = computed(() => tree.value?.canManage ?? false)

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

function statusBadge(v: VideoSummary): { label: string; cls: string } | null {
  if (v.status === 'ready') return null
  if (v.status === 'failed') return { label: 'Failed', cls: 'danger' }
  return { label: v.status === 'uploading' ? 'Uploading' : 'Processing', cls: 'warn' }
}

/* ---------- creator actions ---------- */

interface Selected {
  kind: 'directory' | 'video'
  id: string
  name: string
  directoryId: string
}

const selected = ref<Selected | null>(null)
const sheetOpen = ref(false)
const dialog = ref<'share' | 'access' | 'rename' | 'move' | 'deleteVideo' | 'deleteDir' | 'newFolder' | null>(null)
const busy = ref(false)

const sheetActions: SheetAction[] = [
  { key: 'share', label: 'Share (QR code)', icon: 'share' },
  { key: 'access', label: 'View / edit access', icon: 'users' },
  { key: 'rename', label: 'Rename', icon: 'edit' },
  { key: 'move', label: 'Move', icon: 'move' },
  { key: 'delete', label: 'Delete', icon: 'trash', danger: true },
]

function openSheet(kind: 'directory' | 'video', id: string, name: string, directoryId: string) {
  selected.value = { kind, id, name, directoryId }
  sheetOpen.value = true
}

function pick(key: string) {
  sheetOpen.value = false
  if (!selected.value) return
  if (key === 'delete') dialog.value = selected.value.kind === 'video' ? 'deleteVideo' : 'deleteDir'
  else dialog.value = key as typeof dialog.value
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

function deleteVideo() {
  const s = selected.value
  if (!s) return
  void run(() => api.videos.remove(s.id))
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
  <p v-if="error" class="error">{{ error }}</p>
  <p v-if="loading && !tree" class="muted">Loading…</p>

  <template v-if="tree && current">
    <h2 v-if="crumbs.length <= 1" :style="tree.creator.own ? 'font-weight: 700' : ''">{{ tree.creator.name }}</h2>

    <nav v-if="crumbs.length > 1" class="crumbs" aria-label="Folder path">
      <template v-for="(c, i) in crumbs" :key="c.id">
        <button v-if="i < crumbs.length - 1" type="button" @click="openDir(c.id)">{{ dirName(c) }}</button>
        <span v-else class="current">{{ dirName(c) }}</span>
        <Icon v-if="i < crumbs.length - 1" name="chevron-right" style="width: 14px; height: 14px" />
      </template>
    </nav>

    <div v-if="canManage" class="page-head">
      <span class="muted small">You manage this collection</span>
      <span style="flex: 1" />
      <button type="button" class="btn" @click="dialog = 'newFolder'"><Icon name="folder-plus" /> New folder</button>
      <button
        type="button"
        class="iconbtn"
        aria-label="Folder options"
        @click="openSheet('directory', current.id, dirName(current), current.parentId ?? current.id)"
      >
        <Icon name="more" />
      </button>
    </div>

    <div v-if="current.children.length === 0 && current.videos.length === 0" class="empty">
      <p>This folder is empty.</p>
      <RouterLink v-if="canManage" class="btn primary" :to="{ name: 'upload', query: { dir: current.id } }">
        Upload a video
      </RouterLink>
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
        <span class="lead"><Icon name="folder" /></span>
        <div class="grow">
          <div class="title">{{ d.name }}</div>
          <div class="sub">
            {{ d.children.length }} folder{{ d.children.length === 1 ? '' : 's' }} · {{ d.videos.length }} video{{
              d.videos.length === 1 ? '' : 's'
            }}
          </div>
        </div>
        <button
          v-if="canManage"
          type="button"
          class="iconbtn"
          aria-label="Options"
          @click.stop="openSheet('directory', d.id, d.name, current.id)"
        >
          <Icon name="more" />
        </button>
        <span v-else class="lead"><Icon name="chevron-right" /></span>
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
        <span class="lead"><Icon name="video" /></span>
        <div class="grow">
          <div class="title">{{ v.title }}</div>
          <div class="sub">
            <span v-if="statusBadge(v)" class="badge" :class="statusBadge(v)?.cls">{{ statusBadge(v)?.label }}</span>
            <template v-else>{{ formatDuration(v.durationSec) }}</template>
          </div>
        </div>
        <button
          v-if="canManage"
          type="button"
          class="iconbtn"
          aria-label="Options"
          @click.stop="openSheet('video', v.id, v.title, v.directoryId)"
        >
          <Icon name="more" />
        </button>
      </li>
    </ul>

    <!-- creator dialogs -->
    <ActionSheet
      :open="sheetOpen"
      :title="selected?.name ?? ''"
      :actions="selected?.kind === 'directory' && selected.id === tree.root.id ? sheetActions.slice(0, 2) : sheetActions"
      @close="sheetOpen = false"
      @pick="pick"
    />
    <ShareDialog :open="dialog === 'share'" :target="shareTarget" :name="selected?.name ?? ''" @close="dialog = null" />
    <AccessDialog :open="dialog === 'access'" :target="shareTarget" :name="selected?.name ?? ''" @close="dialog = null" />
    <TextPromptDialog
      :open="dialog === 'rename'"
      title="Rename"
      label="New name"
      :initial="selected?.name"
      confirm-label="Rename"
      @close="dialog = null"
      @submit="rename"
    />
    <TextPromptDialog
      :open="dialog === 'newFolder'"
      title="New folder"
      label="Folder name"
      confirm-label="Create"
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
    <ConfirmDialog
      :open="dialog === 'deleteVideo'"
      title="Delete video"
      :message="`Delete “${selected?.name ?? ''}”? It is removed from Vimeo as well. This cannot be undone.`"
      confirm-label="Delete"
      danger
      :busy="busy"
      @close="dialog = null"
      @confirm="deleteVideo"
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

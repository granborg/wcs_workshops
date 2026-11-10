import { computed, ref } from 'vue'
import { Upload } from 'tus-js-client'
import type { UploadTicket, VideoSummary } from '@/types'
import { api } from '@/services/api'

export type UploadPhase = 'idle' | 'reserving' | 'uploading' | 'finishing' | 'done'

/**
 * Sends a file to Vimeo in three steps: get a ticket from the API, send the bytes with tus, then tell the API
 * the upload is complete. `getTicket` chooses between a new video and an existing one (a placeholder).
 */
export function useVideoUpload() {
  const phase = ref<UploadPhase>('idle')
  const progress = ref(0)
  const busy = computed(() => phase.value !== 'idle' && phase.value !== 'done')

  /** Throws on failure and sets the phase back to idle. */
  async function upload(file: File, getTicket: (sizeBytes: number) => Promise<UploadTicket>): Promise<VideoSummary> {
    progress.value = 0
    phase.value = 'reserving'
    try {
      const ticket = await getTicket(file.size)
      phase.value = 'uploading'
      await new Promise<void>((resolve, reject) => {
        new Upload(file, {
          uploadUrl: ticket.uploadLink,
          chunkSize: 50 * 1024 * 1024,
          retryDelays: [0, 1000, 3000, 5000],
          onError: reject,
          onProgress: (sent, total) => {
            progress.value = total > 0 ? Math.round((sent / total) * 100) : 0
          },
          onSuccess: () => resolve(),
        }).start()
      })
      phase.value = 'finishing'
      const video = await api.videos.complete(ticket.videoId)
      phase.value = 'done'
      return video
    } catch (err) {
      phase.value = 'idle'
      throw err
    }
  }

  return { phase, progress, busy, upload }
}

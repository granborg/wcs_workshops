import { onScopeDispose, ref, watch, type WatchSource } from 'vue'

/**
 * Follows `source`, but turns true only after it stayed true for `delayMs`. It turns false at once.
 * Use it for skeletons and spinners: fast loads show nothing, so the page does not flicker.
 */
export function useDelayedFlag(source: WatchSource<boolean>, delayMs = 500) {
  const shown = ref(false)
  let timer: ReturnType<typeof setTimeout> | undefined

  watch(
    source,
    (on) => {
      clearTimeout(timer)
      if (on) timer = setTimeout(() => (shown.value = true), delayMs)
      else shown.value = false
    },
    { immediate: true },
  )
  onScopeDispose(() => clearTimeout(timer))

  return shown
}

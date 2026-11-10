import { ref, watch } from 'vue'

/**
 * Light, dark, or follow the device. The choice is kept per device in localStorage.
 * Dark mode is the `app-dark` class on <html>: PrimeVue (theme.ts) and style.css both key off it.
 */
export type ColorMode = 'light' | 'dark' | 'system'

const KEY = 'wcs.colorMode'
export const DARK_CLASS = 'app-dark'

function read(): ColorMode {
  try {
    const v = localStorage.getItem(KEY)
    if (v === 'light' || v === 'dark' || v === 'system') return v
  } catch {
    // Storage blocked (private mode, disabled site data): fall back to the device setting.
  }
  return 'system'
}

const media = window.matchMedia('(prefers-color-scheme: dark)')

export const colorMode = ref<ColorMode>(read())

function apply() {
  const dark = colorMode.value === 'dark' || (colorMode.value === 'system' && media.matches)
  const root = document.documentElement
  root.classList.toggle(DARK_CLASS, dark)
  root.style.colorScheme = dark ? 'dark' : 'light'
}

/** Applies the saved mode and keeps it in sync. Call once before the app mounts to avoid a flash. */
export function initColorMode() {
  apply()
  media.addEventListener('change', apply)
  watch(colorMode, (mode) => {
    try {
      localStorage.setItem(KEY, mode)
    } catch {
      // Not saved; the choice still applies until the page reloads.
    }
    apply()
  })
}

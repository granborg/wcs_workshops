import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

// Aura with a blue primary instead of the default emerald. Light or dark follows the device setting.
const WcsPreset = definePreset(Aura, {
  semantic: {
    primary: {
      50: '{blue.50}',
      100: '{blue.100}',
      200: '{blue.200}',
      300: '{blue.300}',
      400: '{blue.400}',
      500: '{blue.500}',
      600: '{blue.600}',
      700: '{blue.700}',
      800: '{blue.800}',
      900: '{blue.900}',
      950: '{blue.950}',
    },
    // Aura's light primary is the 500 shade: white text on blue.500 is 3.7:1, below WCAG AA (4.5:1).
    // blue.600 gives 5.2:1. Dark mode keeps Aura's default (dark text on blue.400, 7:1).
    colorScheme: {
      light: {
        primary: {
          color: '{primary.600}',
          contrastColor: '#ffffff',
          hoverColor: '{primary.700}',
          activeColor: '{primary.800}',
        },
      },
    },
  },
  components: {
    // The breadcrumb sits on the page background, not on a card.
    breadcrumb: { root: { background: 'transparent', padding: '0 0 0.5rem' } },
  },
})

export const theme = {
  preset: WcsPreset,
  // Dark mode follows the class that services/colorMode.ts sets on <html> (light, dark, or device setting).
  options: { darkModeSelector: '.app-dark' },
}

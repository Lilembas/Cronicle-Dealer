import { definePreset } from '@primevue/themes'
import Aura from '@primevue/themes/aura'

const CroniclePreset = definePreset(Aura, {
  semantic: {
    colorScheme: {
      light: {
        primary: {
          color: '{blue.500}',
          contrastColor: '#ffffff',
          hoverColor: '{blue.600}',
          activeColor: '{blue.700}'
        },
        highlight: {
          background: '{blue.50}',
          focusBackground: '{blue.100}',
          color: '{blue.700}',
          focusColor: '{blue.800}'
        }
      },
      dark: {
        primary: {
          color: '{blue.400}',
          contrastColor: '{surface.900}',
          hoverColor: '{blue.300}',
          activeColor: '{blue.200}'
        },
        highlight: {
          background: 'color-mix(in srgb, {blue.400}, transparent 84%)',
          focusBackground: 'color-mix(in srgb, {blue.400}, transparent 76%)',
          color: 'rgba(255,255,255,.87)',
          focusColor: 'rgba(255,255,255,.87)'
        }
      }
    }
  },
  components: {
    card: {
      borderRadius: '12px',
      shadow: '0 1px 3px rgba(0,0,0,0.04), 0 1px 2px rgba(0,0,0,0.06)'
    },
    button: {
      borderRadius: '8px',
      paddingX: '0.875rem',
      paddingY: '0.5rem'
    },
    datatable: {
      bodyRow: {
        transitionDuration: '0.15s'
      }
    },
    tabs: {
      tab: {
        paddingX: '1rem',
        paddingY: '0.625rem',
        fontWeight: '500',
        transitionDuration: '0.2s'
      }
    }
  }
})

export default CroniclePreset

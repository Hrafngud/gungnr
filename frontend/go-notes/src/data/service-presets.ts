// src/data/service-presets.ts
import servicePresetsJson from '@/data/service-presets.json'
import type { ServicePreset } from '@/types/service-preset'

// Type assertion – safe because we control the JSON shape
export const servicePresets = servicePresetsJson as ServicePreset[]
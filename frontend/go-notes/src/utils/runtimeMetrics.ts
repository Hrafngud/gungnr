export const clampPercent = (value: number | null | undefined) => {
  if (typeof value !== 'number' || Number.isNaN(value)) return 0
  if (value < 0) return 0
  if (value > 100) return 100
  return value
}

export const formatPercent = (value: number | null | undefined) => `${clampPercent(value).toFixed(1)}%`

export const formatBytes = (bytes: number | null | undefined) => {
  if (typeof bytes !== 'number' || !Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let value = bytes
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  const rounded = value >= 10 ? value.toFixed(1) : value.toFixed(2)
  return `${rounded} ${units[index]}`
}

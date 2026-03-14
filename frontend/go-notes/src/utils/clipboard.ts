const DEFAULT_NON_COPYABLE_VALUES = [
  '',
  '—',
  '--',
  'unset',
  'unknown',
  'unavailable',
  'n/a',
]

export async function writeTextToClipboard(payload: string) {
  if (navigator?.clipboard?.writeText) {
    await navigator.clipboard.writeText(payload)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = payload
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
}

export function isCopyValueAllowed(
  value: string,
  nonCopyableValues: string[] = DEFAULT_NON_COPYABLE_VALUES,
) {
  const normalized = value.trim().toLowerCase()
  return !nonCopyableValues.includes(normalized)
}

export type CloudflarePreflightStatus =
  | 'ok'
  | 'warning'
  | 'error'
  | 'missing'
  | 'skipped'

export interface CloudflareCheck {
  status: CloudflarePreflightStatus
  detail?: string
}

export interface CloudflarePreflight {
  token: CloudflareCheck
  account: CloudflareCheck
  zone: CloudflareCheck
  tunnel: CloudflareCheck
  tunnelRef?: string
  tunnelRefType?: string
}

export interface CloudflareZone {
  id: string
  name: string
}

export interface CloudflareZonesResponse {
  zones: CloudflareZone[]
}

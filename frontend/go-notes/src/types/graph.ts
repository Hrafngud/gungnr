export type NodeEdgeGraphTone =
  | 'neutral'
  | 'ok'
  | 'warn'
  | 'error'
  | 'inbound'
  | 'outbound'
  | 'allow'
  | 'running'
  | 'degraded'
  | 'failed'
  | 'missing'
  | 'unknown'
  | 'group'
  | 'service'
  | 'project'

export interface NodeEdgeGraphNode {
  id: string
  label: string
  subtitle?: string
  tone?: NodeEdgeGraphTone
}

export interface NodeEdgeGraphEdge {
  id: string
  from: string
  to: string
  label?: string
  tone?: NodeEdgeGraphTone
}

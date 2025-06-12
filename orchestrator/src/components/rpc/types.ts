export interface RPCParam {
  name: string
  type: string
  required: boolean
  description: string
  default?: string | number | boolean | null
}

export interface RPCMethod {
  name: string
  description: string
  method: string
  params: RPCParam[]
  example: {
    jsonrpc: string
    method: string
    params: unknown[]
    id: number
  }
}

export interface RPCCategory {
  name: string
  methods: Record<string, RPCMethod>
}

export interface RPCCall {
  id: string
  timestamp: number
  method: string
  params: unknown[]
  response?: unknown
  error?: string
  duration?: number
}

export interface Environment {
  name: string
  url: string
  auth?: {
    username: string
    password: string
  }
  headers?: Record<string, string>
}

export interface GRPCMethod {
  name: string
  description: string
  method: string
  streaming?: boolean
  params: RPCParam[]
  example: string
}

export interface GRPCCategory {
  name: string
  methods: Record<string, GRPCMethod>
} 
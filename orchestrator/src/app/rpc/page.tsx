"use client"

import { HistoryDrawer } from "@/components/rpc/history-drawer"
import { RequestBuilder } from "@/components/rpc/request-builder"
import { ResponseViewer } from "@/components/rpc/response-viewer"
import { RPCSidebar } from "@/components/rpc/rpc-sidebar"
import { SettingsDialog } from "@/components/rpc/settings-dialog"
import { GRPC_TEMPLATES, RPC_TEMPLATES } from "@/components/rpc/templates"
import { Environment, GRPCMethod, RPCCall, RPCMethod } from "@/components/rpc/types"
import { buildGRPCCommand, buildRPCRequest } from "@/components/rpc/utils"
import { TooltipProvider } from "@/components/ui/tooltip"
import { useEffect, useState } from "react"

export default function RPCConsolePage() {
  const [selectedCategory, setSelectedCategory] = useState("blocks")
  const [selectedMethod, setSelectedMethod] = useState("getBlock")
  const [selectedProtocol, setSelectedProtocol] = useState<"rpc" | "grpc">("rpc")
  const [requestBody, setRequestBody] = useState("")
  const [response, setResponse] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [history, setHistory] = useState<RPCCall[]>([])
  const [environments] = useState<Environment[]>([
    {
      name: "HTTP",
      url: "http://65.21.170.85"
    }
  ])
  const [selectedEnvironment, setSelectedEnvironment] = useState(0)
  const [paramValues, setParamValues] = useState<Record<string, string>>({})

  // Initialize with first method
  useEffect(() => {
    updateRequestBody("blocks", "getBlock", "rpc", {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const updateRequestBody = (
    category: string, 
    method: string, 
    protocol: "rpc" | "grpc",
    currentParamValues = paramValues
  ) => {
    if (protocol === "rpc") {
      const template = RPC_TEMPLATES[category]?.methods[method]
      if (template) {
        setRequestBody(buildRPCRequest(template, currentParamValues))
      }
    } else {
      const template = GRPC_TEMPLATES[category]?.methods[method]
      if (template) {
        setRequestBody(buildGRPCCommand(template, currentParamValues))
      }
    }
  }

  const handleMethodSelect = (category: string, method: string, protocol: "rpc" | "grpc") => {
    setSelectedCategory(category)
    setSelectedMethod(method)
    setSelectedProtocol(protocol)
    setParamValues({})
    updateRequestBody(category, method, protocol, {})
  }

  const handleParamChange = (paramName: string, value: string) => {
    const newValues = { ...paramValues, [paramName]: value }
    setParamValues(newValues)
    updateRequestBody(selectedCategory, selectedMethod, selectedProtocol, newValues)
  }

  const executeRPC = async () => {
    if (selectedProtocol === "grpc") {
      // For gRPC, we just show the command - actual execution would require grpcurl
      setResponse("gRPC command ready to execute. Copy and run in terminal with grpcurl installed.")
      return
    }

    setIsLoading(true)
    const startTime = Date.now()
    
    try {
      const parsedBody = JSON.parse(requestBody)
      const env = environments[selectedEnvironment]
      
      const response = await fetch(env.url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(env.auth && {
            "Authorization": `Basic ${btoa(`${env.auth.username}:${env.auth.password}`)}`
          }),
          ...env.headers
        },
        body: requestBody
      })
      
      const result = await response.json()
      const duration = Date.now() - startTime
      
      setResponse(JSON.stringify(result, null, 2))
      
      // Add to history
      const call: RPCCall = {
        id: Date.now().toString(),
        timestamp: Date.now(),
        method: parsedBody.method,
        params: parsedBody.params || [],
        response: result,
        duration
      }
      
      setHistory(prev => [call, ...prev.slice(0, 49)]) // Keep last 50 calls
      
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : "Unknown error"
      setResponse(JSON.stringify({ error: errorMessage }, null, 2))
      
      const call: RPCCall = {
        id: Date.now().toString(),
        timestamp: Date.now(),
        method: "unknown",
        params: [],
        error: errorMessage,
        duration: Date.now() - startTime
      }
      
      setHistory(prev => [call, ...prev.slice(0, 49)])
    } finally {
      setIsLoading(false)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
  }

  const loadFromHistory = (call: RPCCall) => {
    const body = {
      jsonrpc: "2.0",
      method: call.method,
      params: call.params,
      id: 1
    }
    setRequestBody(JSON.stringify(body, null, 2))
    if (call.response) {
      setResponse(JSON.stringify(call.response, null, 2))
    }
  }

  const exportHistory = () => {
    const data = JSON.stringify(history, null, 2)
    const blob = new Blob([data], { type: "application/json" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `rpc-history-${new Date().toISOString().split("T")[0]}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  const getCurrentTemplate = (): RPCMethod | GRPCMethod | null => {
    if (selectedProtocol === "rpc") {
      return RPC_TEMPLATES[selectedCategory]?.methods[selectedMethod] || null
    } else {
      return GRPC_TEMPLATES[selectedCategory]?.methods[selectedMethod] || null
    }
  }

  return (
    <TooltipProvider>
      <div className="flex flex-col bg-background h-full">
        {/* Header */}
        <div className="border-b p-4">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold">Old Faithful RPC Console</h1>
              <p className="text-sm text-muted-foreground">
                Execute Solana RPC calls and Old Faithful gRPC methods
              </p>
            </div>
            <div className="flex items-center gap-2">
              <HistoryDrawer
                history={history}
                onLoadFromHistory={loadFromHistory}
                onExportHistory={exportHistory}
              />
              <SettingsDialog
                environments={environments}
                selectedEnvironment={selectedEnvironment}
                onEnvironmentChange={setSelectedEnvironment}
              />
            </div>
          </div>
        </div>

        {/* Main Content with Sidebar */}
        <div className="flex-1 flex overflow-hidden">
          {/* Sidebar */}
          <RPCSidebar
            selectedMethod={selectedMethod}
            selectedCategory={selectedCategory}
            selectedProtocol={selectedProtocol}
            onMethodSelect={handleMethodSelect}
          />

          {/* Content Area */}
          <div className="flex-1 flex gap-4 p-4 overflow-hidden">
            <RequestBuilder
              currentTemplate={getCurrentTemplate()}
              selectedProtocol={selectedProtocol}
              paramValues={paramValues}
              requestBody={requestBody}
              isLoading={isLoading}
              onParamChange={handleParamChange}
              onRequestBodyChange={setRequestBody}
              onExecute={executeRPC}
              onCopyRequest={() => copyToClipboard(requestBody)}
            />
            
            <ResponseViewer
              response={response}
              environments={environments}
              selectedEnvironment={selectedEnvironment}
              onCopyResponse={() => copyToClipboard(response)}
            />
          </div>
        </div>
      </div>
    </TooltipProvider>
  )
}

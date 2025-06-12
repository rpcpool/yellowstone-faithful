"use client"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { Textarea } from "@/components/ui/textarea"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { Copy, Info, Play } from "lucide-react"
import { GRPCMethod, RPCMethod } from "./types"

interface RequestBuilderProps {
  currentTemplate: RPCMethod | GRPCMethod | null
  selectedProtocol: "rpc" | "grpc"
  paramValues: Record<string, string>
  requestBody: string
  isLoading: boolean
  onParamChange: (paramName: string, value: string) => void
  onRequestBodyChange: (value: string) => void
  onExecute: () => void
  onCopyRequest: () => void
}

export function RequestBuilder({
  currentTemplate,
  selectedProtocol,
  paramValues,
  requestBody,
  isLoading,
  onParamChange,
  onRequestBodyChange,
  onExecute,
  onCopyRequest
}: RequestBuilderProps) {
  const isGRPC = selectedProtocol === "grpc"
  
  return (
    <Card className="flex-1">
      <CardHeader>
        <CardTitle className="text-lg flex items-center gap-2">
          Request Builder
          {currentTemplate && (
            <Tooltip>
              <TooltipTrigger>
                <Info className="h-4 w-4" />
              </TooltipTrigger>
              <TooltipContent>
                <p>{currentTemplate.description}</p>
              </TooltipContent>
            </Tooltip>
          )}
        </CardTitle>
        <CardDescription>
          Configure parameters and build your {isGRPC ? "gRPC" : "RPC"} request
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Parameters */}
        {currentTemplate?.params && currentTemplate.params.length > 0 && (
          <div className="space-y-3">
            <Label className="text-sm font-medium">Parameters</Label>
            {currentTemplate.params.map((param) => (
              <div key={param.name} className="space-y-1">
                <div className="flex items-center gap-2">
                  <Label className="text-xs">{param.name}</Label>
                  {param.required && (
                    <Badge variant="destructive" className="text-xs">
                      Required
                    </Badge>
                  )}
                  <Badge variant="outline" className="text-xs">
                    {param.type}
                  </Badge>
                  {param.default !== undefined && (
                    <Badge variant="secondary" className="text-xs">
                      Default: {String(param.default)}
                    </Badge>
                  )}
                </div>
                <Input
                  placeholder={
                    param.default !== undefined
                      ? `${param.description} (default: ${param.default})`
                      : param.description
                  }
                  value={paramValues[param.name] || ""}
                  onChange={(e) => onParamChange(param.name, e.target.value)}
                  className="text-sm"
                />
                <p className="text-xs text-muted-foreground">{param.description}</p>
              </div>
            ))}
            <Separator />
          </div>
        )}

        {/* Raw Request */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label className="text-sm font-medium">
              {isGRPC ? "gRPC Command" : "Raw Request"}
            </Label>
            <Button variant="outline" size="sm" onClick={onCopyRequest}>
              <Copy className="h-4 w-4" />
            </Button>
          </div>
          <Textarea
            value={requestBody}
            onChange={(e) => onRequestBodyChange(e.target.value)}
            className="font-mono text-sm min-h-[200px]"
            placeholder={
              isGRPC
                ? "gRPC command will appear here..."
                : "JSON-RPC request body"
            }
            readOnly={isGRPC}
          />
        </div>

        <Button
          onClick={onExecute}
          disabled={isLoading || !requestBody.trim()}
          className="w-full"
        >
          {isLoading ? (
            <>Loading...</>
          ) : (
            <>
              <Play className="h-4 w-4 mr-2" />
              Execute {isGRPC ? "gRPC" : "RPC"} Call
            </>
          )}
        </Button>
      </CardContent>
    </Card>
  )
} 
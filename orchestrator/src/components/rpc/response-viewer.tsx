"use client"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Copy, Info } from "lucide-react"
import { Environment } from "./types"

interface ResponseViewerProps {
  response: string
  environments: Environment[]
  selectedEnvironment: number
  onCopyResponse: () => void
}

export function ResponseViewer({
  response,
  environments,
  selectedEnvironment,
  onCopyResponse
}: ResponseViewerProps) {
  return (
    <Card className="flex-1 flex flex-col min-h-0">
      <CardHeader>
        <CardTitle className="text-lg">Response</CardTitle>
        <CardDescription>
          View the RPC response and any errors
        </CardDescription>
      </CardHeader>
      <CardContent className="flex-1 flex flex-col space-y-4 overflow-y-auto min-h-0">
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label className="text-sm font-medium">Response Body</Label>
            {response && (
              <Button variant="outline" size="sm" onClick={onCopyResponse}>
                <Copy className="h-4 w-4" />
              </Button>
            )}
          </div>
          <Textarea
            value={response}
            readOnly
            className="font-mono text-sm flex-1 min-h-[300px] resize-none overflow-auto"
            placeholder="Response will appear here..."
          />
        </div>

        {/* Environment Info */}
        <Alert>
          <Info className="h-4 w-4" />
          <AlertDescription>
            <strong>Endpoint:</strong>{" "}
            {environments[selectedEnvironment]?.url || "Not configured"}
          </AlertDescription>
        </Alert>
      </CardContent>
    </Card>
  )
} 
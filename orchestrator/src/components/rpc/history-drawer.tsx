"use client"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Drawer, DrawerContent, DrawerDescription, DrawerHeader, DrawerTitle, DrawerTrigger } from "@/components/ui/drawer"
import { Download, History } from "lucide-react"
import { RPCCall } from "./types"

interface HistoryDrawerProps {
  history: RPCCall[]
  onLoadFromHistory: (call: RPCCall) => void
  onExportHistory: () => void
}

export function HistoryDrawer({ history, onLoadFromHistory, onExportHistory }: HistoryDrawerProps) {
  return (
    <Drawer direction="right">
      <DrawerTrigger asChild>
        <Button variant="outline" size="sm">
          <History className="h-4 w-4 mr-2" />
          History ({history.length})
        </Button>
      </DrawerTrigger>
      <DrawerContent className="h-full w-[500px] ml-auto">
        <DrawerHeader className="border-b">
          <DrawerTitle>RPC Call History</DrawerTitle>
          <DrawerDescription>
            Recent RPC calls and their responses
          </DrawerDescription>
          <div className="flex justify-end pt-2">
            <Button onClick={onExportHistory} size="sm" variant="outline">
              <Download className="h-4 w-4 mr-2" />
              Export
            </Button>
          </div>
        </DrawerHeader>
        
        <div className="flex-1 overflow-y-auto p-4">
          <div className="space-y-3">
            {history.length === 0 ? (
              <div className="text-center text-muted-foreground py-8">
                No RPC calls in history yet
              </div>
            ) : (
              history.map((call) => (
                <Card 
                  key={call.id} 
                  className="cursor-pointer hover:bg-muted/50 transition-colors" 
                  onClick={() => onLoadFromHistory(call)}
                >
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Badge variant="outline" className="text-xs">
                          {call.method}
                        </Badge>
                        {call.error && (
                          <Badge variant="destructive" className="text-xs">
                            Error
                          </Badge>
                        )}
                        {call.duration && (
                          <Badge variant="secondary" className="text-xs">
                            {call.duration}ms
                          </Badge>
                        )}
                      </div>
                      <span className="text-xs text-muted-foreground">
                        {new Date(call.timestamp).toLocaleTimeString()}
                      </span>
                    </div>
                  </CardHeader>
                  <CardContent className="pt-0">
                    <div className="text-xs space-y-1">
                      <div>
                        <span className="font-medium">Params:</span>{" "}
                        <span className="text-muted-foreground">
                          {JSON.stringify(call.params).length > 50
                            ? JSON.stringify(call.params).substring(0, 50) + "..."
                            : JSON.stringify(call.params)}
                        </span>
                      </div>
                      {call.error && (
                        <div className="text-destructive">
                          <span className="font-medium">Error:</span> {call.error}
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              ))
            )}
          </div>
        </div>
      </DrawerContent>
    </Drawer>
  )
} 
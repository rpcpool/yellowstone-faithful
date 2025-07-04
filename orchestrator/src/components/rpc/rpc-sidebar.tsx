"use client"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Separator } from "@/components/ui/separator"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import { ChevronDown, Server, Zap } from "lucide-react"
import { useState } from "react"
import { GRPC_TEMPLATES, RPC_TEMPLATES } from "./templates"

interface RPCSidebarProps {
  selectedMethod: string
  selectedCategory: string
  selectedProtocol: "rpc" | "grpc"
  onMethodSelect: (category: string, method: string, protocol: "rpc" | "grpc") => void
}

export function RPCSidebar({ 
  selectedMethod, 
  selectedCategory, 
  selectedProtocol,
  onMethodSelect 
}: RPCSidebarProps) {
  const [openCategories, setOpenCategories] = useState<Record<string, boolean>>({
    "rpc-blocks": true,
    "grpc-blocks": false,
  })

  const toggleCategory = (categoryKey: string) => {
    setOpenCategories(prev => ({
      ...prev,
      [categoryKey]: !prev[categoryKey]
    }))
  }

  return (
    <div className="w-80 border-r bg-muted/30 flex flex-col h-full">
      <div className="p-4 border-b flex-shrink-0">
        <h2 className="font-semibold text-lg">API Methods</h2>
        <p className="text-sm text-muted-foreground">
          Solana RPC & Old Faithful gRPC
        </p>
      </div>
      
      <div className="flex-1 overflow-y-auto min-h-0">
        <div className="p-4 space-y-4">
          {/* RPC Methods */}
          <div className="space-y-2">
            <div className="flex items-center gap-2 mb-3">
              <Server className="h-4 w-4" />
              <span className="font-medium text-sm">JSON-RPC Methods</span>
            </div>
            
            {Object.entries(RPC_TEMPLATES).map(([categoryKey, category]) => {
              const fullCategoryKey = `rpc-${categoryKey}`
              const isOpen = openCategories[fullCategoryKey]
              
              return (
                <Collapsible 
                  key={fullCategoryKey} 
                  open={isOpen}
                  onOpenChange={() => toggleCategory(fullCategoryKey)}
                >
                  <CollapsibleTrigger className="flex items-center justify-between w-full p-2 hover:bg-muted rounded-md text-left">
                    <span className="text-sm font-medium">{category.name}</span>
                    <ChevronDown className={cn("h-4 w-4 transition-transform", isOpen && "rotate-180")} />
                  </CollapsibleTrigger>
                  <CollapsibleContent className="space-y-1 mt-1 ml-2">
                    {Object.entries(category.methods).map(([methodKey, method]) => (
                      <Tooltip key={methodKey}>
                        <TooltipTrigger asChild>
                          <Button
                            variant={
                              selectedMethod === methodKey && 
                              selectedCategory === categoryKey && 
                              selectedProtocol === "rpc" 
                                ? "default" 
                                : "ghost"
                            }
                            className="w-full justify-start text-left h-8 p-2 text-xs"
                            onClick={() => onMethodSelect(categoryKey, methodKey, "rpc")}
                          >
                            <span className="font-medium">{method.name}</span>
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="right" className="max-w-xs">
                          <p>{method.description}</p>
                        </TooltipContent>
                      </Tooltip>
                    ))}
                  </CollapsibleContent>
                </Collapsible>
              )
            })}
          </div>

          <Separator />

          {/* gRPC Methods */}
          <div className="space-y-2">
            <div className="flex items-center gap-2 mb-3">
              <Zap className="h-4 w-4" />
              <span className="font-medium text-sm">gRPC Methods</span>
            </div>
            
            {Object.entries(GRPC_TEMPLATES).map(([categoryKey, category]) => {
              const fullCategoryKey = `grpc-${categoryKey}`
              const isOpen = openCategories[fullCategoryKey]
              
              return (
                <Collapsible 
                  key={fullCategoryKey} 
                  open={isOpen}
                  onOpenChange={() => toggleCategory(fullCategoryKey)}
                >
                  <CollapsibleTrigger className="flex items-center justify-between w-full p-2 hover:bg-muted rounded-md text-left">
                    <span className="text-sm font-medium">{category.name}</span>
                    <ChevronDown className={cn("h-4 w-4 transition-transform", isOpen && "rotate-180")} />
                  </CollapsibleTrigger>
                  <CollapsibleContent className="space-y-1 mt-1 ml-2">
                    {Object.entries(category.methods).map(([methodKey, method]) => (
                      <Tooltip key={methodKey}>
                        <TooltipTrigger asChild>
                          <Button
                            variant={
                              selectedMethod === methodKey && 
                              selectedCategory === categoryKey && 
                              selectedProtocol === "grpc" 
                                ? "default" 
                                : "ghost"
                            }
                            className="w-full justify-start text-left h-8 p-2 text-xs"
                            onClick={() => onMethodSelect(categoryKey, methodKey, "grpc")}
                          >
                            <div className="flex items-center gap-2">
                              <span className="font-medium">{method.name}</span>
                              {method.streaming && (
                                <Badge variant="secondary" className="text-xs">
                                  Stream
                                </Badge>
                              )}
                            </div>
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="right" className="max-w-xs">
                          <p>{method.description}</p>
                        </TooltipContent>
                      </Tooltip>
                    ))}
                  </CollapsibleContent>
                </Collapsible>
              )
            })}
          </div>
        </div>
      </div>
    </div>
  )
} 
"use client"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Settings } from "lucide-react"
import { Environment } from "./types"

interface SettingsDialogProps {
  environments: Environment[]
  selectedEnvironment: number
  onEnvironmentChange: (index: number) => void
}

export function SettingsDialog({
  environments,
  selectedEnvironment,
  onEnvironmentChange
}: SettingsDialogProps) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <Settings className="h-4 w-4 mr-2" />
          Settings
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Environment Settings</DialogTitle>
          <DialogDescription>
            Configure RPC endpoints and authentication
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <Label>Current Environment</Label>
            <Select
              value={selectedEnvironment.toString()}
              onValueChange={(value) => onEnvironmentChange(Number(value))}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {environments.map((env, index) => (
                  <SelectItem key={index} value={index.toString()}>
                    {env.name} - {env.url}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {/* Add environment management UI here */}
        </div>
      </DialogContent>
    </Dialog>
  )
} 
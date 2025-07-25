"use client";

import { useState, useEffect } from "react";
import { useMutation } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";
import { DataSourceType } from "@/generated/prisma";

interface Source {
  id: string;
  name: string;
  type: DataSourceType;
  configuration: Record<string, unknown>;
  enabled: boolean;
}

interface SourceDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  source?: Source | null;
  onSuccess: () => void;
}

const SOURCE_TYPE_LABELS: Record<DataSourceType, string> = {
  S3: "Amazon S3",
  HTTP: "HTTP",
  FILESYSTEM: "Filesystem",
};

export function SourceDialog({
  open,
  onOpenChange,
  source,
  onSuccess,
}: SourceDialogProps) {
  const [formData, setFormData] = useState<{
    name: string;
    type: DataSourceType;
    enabled: boolean;
    configuration: Record<string, unknown>;
  }>({
    name: "",
    type: DataSourceType.HTTP,
    enabled: true,
    configuration: {},
  });

  useEffect(() => {
    if (source) {
      setFormData({
        name: source.name,
        type: source.type,
        enabled: source.enabled,
        configuration: source.configuration,
      });
    } else {
      setFormData({
        name: "",
        type: DataSourceType.HTTP,
        enabled: true,
        configuration: {},
      });
    }
  }, [source]);

  const createMutation = useMutation({
    mutationFn: async (data: typeof formData) => {
      const response = await fetch("/api/sources", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || "Failed to create source");
      }
      return response.json();
    },
    onSuccess: () => {
      toast.success("Source created successfully");
      onSuccess();
    },
    onError: (error) => {
      toast.error(`Failed to create source: ${error.message}`);
    },
  });

  const updateMutation = useMutation({
    mutationFn: async (data: typeof formData) => {
      const response = await fetch(`/api/sources/${source?.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || "Failed to update source");
      }
      return response.json();
    },
    onSuccess: () => {
      toast.success("Source updated successfully");
      onSuccess();
    },
    onError: (error) => {
      toast.error(`Failed to update source: ${error.message}`);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (source) {
      updateMutation.mutate(formData);
    } else {
      createMutation.mutate(formData);
    }
  };

  const renderConfigurationFields = () => {
    switch (formData.type) {
      case DataSourceType.S3:
        return (
          <>
            <div className="grid gap-2">
              <Label htmlFor="bucket">Bucket</Label>
              <Input
                id="bucket"
                value={(formData.configuration.bucket as string) || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    configuration: {
                      ...formData.configuration,
                      bucket: e.target.value,
                    },
                  })
                }
                placeholder="my-bucket"
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="region">Region</Label>
              <Input
                id="region"
                value={(formData.configuration.region as string) || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    configuration: {
                      ...formData.configuration,
                      region: e.target.value,
                    },
                  })
                }
                placeholder="us-east-1"
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="endpoint">Endpoint (Optional)</Label>
              <Input
                id="endpoint"
                value={(formData.configuration.endpoint as string) || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    configuration: {
                      ...formData.configuration,
                      endpoint: e.target.value,
                    },
                  })
                }
                placeholder="https://s3.amazonaws.com"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="accessKeyId">Access Key ID (Optional)</Label>
              <Input
                id="accessKeyId"
                type="password"
                value={(formData.configuration.accessKeyId as string) || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    configuration: {
                      ...formData.configuration,
                      accessKeyId: e.target.value,
                    },
                  })
                }
                placeholder="AKIAIOSFODNN7EXAMPLE"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="secretAccessKey">Secret Access Key (Optional)</Label>
              <Input
                id="secretAccessKey"
                type="password"
                value={(formData.configuration.secretAccessKey as string) || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    configuration: {
                      ...formData.configuration,
                      secretAccessKey: e.target.value,
                    },
                  })
                }
                placeholder="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
              />
            </div>
          </>
        );

      case DataSourceType.HTTP:
        return (
          <>
            <div className="grid gap-2">
              <Label htmlFor="host">Host</Label>
              <Input
                id="host"
                value={(formData.configuration.host as string) || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    configuration: {
                      ...formData.configuration,
                      host: e.target.value,
                    },
                  })
                }
                placeholder="https://example.com"
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="path">Path</Label>
              <Input
                id="path"
                value={(formData.configuration.path as string) || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    configuration: {
                      ...formData.configuration,
                      path: e.target.value,
                    },
                  })
                }
                placeholder="/data"
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="username">Username (Optional)</Label>
              <Input
                id="username"
                value={(formData.configuration.username as string) || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    configuration: {
                      ...formData.configuration,
                      username: e.target.value,
                    },
                  })
                }
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">Password (Optional)</Label>
              <Input
                id="password"
                type="password"
                value={(formData.configuration.password as string) || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    configuration: {
                      ...formData.configuration,
                      password: e.target.value,
                    },
                  })
                }
              />
            </div>
          </>
        );

      case DataSourceType.FILESYSTEM:
        return (
          <div className="grid gap-2">
            <Label htmlFor="basePath">Base Path</Label>
            <Input
              id="basePath"
              value={(formData.configuration.basePath as string) || ""}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  configuration: {
                    ...formData.configuration,
                    basePath: e.target.value,
                  },
                })
              }
              placeholder="/data/cars"
              required
            />
          </div>
        );


      default:
        return null;
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>
              {source ? "Edit Source" : "Create New Source"}
            </DialogTitle>
            <DialogDescription>
              {source
                ? "Update the source configuration below."
                : "Configure a new data source for epoch data."}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.target.value })
                }
                placeholder="My Data Source"
                required
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="type">Type</Label>
              <Select
                value={formData.type}
                onValueChange={(value) =>
                  setFormData({
                    ...formData,
                    type: value as DataSourceType,
                    configuration: {}, // Reset configuration when type changes
                  })
                }
                disabled={!!source} // Don't allow changing type when editing
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(SOURCE_TYPE_LABELS).map(([value, label]) => (
                    <SelectItem key={value} value={value}>
                      {label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-center space-x-2">
              <Switch
                id="enabled"
                checked={formData.enabled}
                onCheckedChange={(checked: boolean) =>
                  setFormData({ ...formData, enabled: checked })
                }
              />
              <Label htmlFor="enabled">Enabled</Label>
            </div>

            <div className="space-y-4">
              <h4 className="text-sm font-medium">Configuration</h4>
              {renderConfigurationFields()}
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={createMutation.isPending || updateMutation.isPending}
            >
              {createMutation.isPending || updateMutation.isPending
                ? "Saving..."
                : source
                ? "Update"
                : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
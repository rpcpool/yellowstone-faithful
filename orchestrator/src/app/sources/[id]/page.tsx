"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ArrowLeft, Power, PowerOff, HardDrive, Globe, Database, Edit, Trash2 } from "lucide-react";
import Link from "next/link";
import { DataSourceType } from "@/generated/prisma";
import { toast } from "sonner";
import { SourceDialog } from "@/components/sources/source-dialog";
import { DeleteSourceDialog } from "@/components/sources/delete-source-dialog";
import { RefreshSourceButton } from "@/components/sources/refresh-source-button";

interface Source {
  id: string;
  name: string;
  type: DataSourceType;
  configuration: Record<string, unknown>;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

interface EpochWithStatus {
  epochNumber: number;
  status: "NotProcessed" | "Processing" | "Indexed" | "Complete";
  indexes: {
    type: string;
    updatedAt: string;
  }[];
  lastUpdated: string;
}

interface EpochsResponse {
  epochs: EpochWithStatus[];
  statistics: {
    total: number;
    byStatus: Record<string, number>;
  };
}

const SOURCE_TYPE_LABELS: Record<DataSourceType, string> = {
  S3: "Amazon S3",
  HTTP: "HTTP",
  FILESYSTEM: "Filesystem",
};

const SOURCE_TYPE_COLORS: Record<DataSourceType, string> = {
  S3: "bg-orange-100 text-orange-800 border-orange-200 dark:bg-orange-900/20 dark:text-orange-300 dark:border-orange-800",
  HTTP: "bg-green-100 text-green-800 border-green-200 dark:bg-green-900/20 dark:text-green-300 dark:border-green-800",
  FILESYSTEM: "bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/20 dark:text-blue-300 dark:border-blue-800",
};

const SOURCE_TYPE_ICONS: Record<DataSourceType, React.ReactNode> = {
  S3: <HardDrive className="h-4 w-4" />,
  HTTP: <Globe className="h-4 w-4" />,
  FILESYSTEM: <Database className="h-4 w-4" />,
};

function getStatusColor(status: string) {
  switch (status) {
    case 'Complete':
      return 'bg-green-500/10 text-green-500 border-green-500/20 dark:bg-green-500/20 dark:text-green-400 dark:border-green-500/30';
    case 'Indexed':
      return 'bg-blue-500/10 text-blue-500 border-blue-500/20 dark:bg-blue-500/20 dark:text-blue-400 dark:border-blue-500/30';
    case 'Processing':
      return 'bg-yellow-500/10 text-yellow-500 border-yellow-500/20 dark:bg-yellow-500/20 dark:text-yellow-400 dark:border-yellow-500/30';
    case 'NotProcessed':
      return 'bg-gray-500/10 text-gray-700 border-gray-500/20 dark:bg-gray-500/20 dark:text-gray-300 dark:border-gray-500/30';
    default:
      return 'bg-gray-500/10 text-gray-700 border-gray-500/20 dark:bg-gray-500/20 dark:text-gray-300 dark:border-gray-500/30';
  }
}

function formatConfiguration(type: DataSourceType, config: Record<string, unknown>) {
  switch (type) {
    case 'S3':
      return (
        <>
          <div>
            <label className="text-sm font-medium text-muted-foreground">Bucket</label>
            <p className="text-lg">{config.bucket as string || 'N/A'}</p>
          </div>
          <div>
            <label className="text-sm font-medium text-muted-foreground">Region</label>
            <p className="text-lg">{config.region as string || 'N/A'}</p>
          </div>
          {config.endpoint && (
            <div>
              <label className="text-sm font-medium text-muted-foreground">Endpoint</label>
              <p className="text-lg font-mono text-sm">{config.endpoint as string}</p>
            </div>
          )}
        </>
      );
    case 'HTTP':
      return (
        <>
          <div>
            <label className="text-sm font-medium text-muted-foreground">Host</label>
            <p className="text-lg font-mono">{config.host as string || 'N/A'}</p>
          </div>
          {config.path && (
            <div>
              <label className="text-sm font-medium text-muted-foreground">Path</label>
              <p className="text-lg font-mono">{config.path as string}</p>
            </div>
          )}
        </>
      );
    case 'FILESYSTEM':
      return (
        <div>
          <label className="text-sm font-medium text-muted-foreground">Base Path</label>
          <p className="text-lg font-mono">{config.basePath as string || 'N/A'}</p>
        </div>
      );
    default:
      return null;
  }
}

export default function SourceDetailPage() {
  const params = useParams();
  const router = useRouter();
  const queryClient = useQueryClient();
  const id = params.id as string;

  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);

  // Fetch source data
  const { data: sourceData, isLoading: sourceLoading, error: sourceError } = useQuery<{ source: Source }>({
    queryKey: ["source", id],
    queryFn: async () => {
      const response = await fetch(`/api/sources/${id}`);
      if (!response.ok) {
        if (response.status === 404) {
          throw new Error("Source not found");
        }
        throw new Error("Failed to fetch source");
      }
      return response.json();
    },
  });

  // Fetch epochs data
  const { data: epochsData, isLoading: epochsLoading, error: epochsError } = useQuery<EpochsResponse>({
    queryKey: ["source-epochs", id],
    queryFn: async () => {
      const response = await fetch(`/api/sources/${id}/epochs`);
      if (!response.ok) {
        throw new Error("Failed to fetch epochs");
      }
      return response.json();
    },
    enabled: !!sourceData,
  });

  // Toggle source mutation
  const toggleSourceMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      const response = await fetch(`/api/sources/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled }),
      });
      if (!response.ok) {
        throw new Error("Failed to update source");
      }
      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["source", id] });
      queryClient.invalidateQueries({ queryKey: ["sources"] });
      toast.success("Source updated successfully");
    },
    onError: (error) => {
      toast.error(`Failed to update source: ${error.message}`);
    },
  });

  const handleToggleSource = () => {
    if (sourceData?.source) {
      toggleSourceMutation.mutate(!sourceData.source.enabled);
    }
  };

  const handleDeleteSuccess = () => {
    router.push("/sources");
  };

  if (sourceLoading) {
    return (
      <div className="container mx-auto py-6">
        <div className="text-center py-8">Loading source details...</div>
      </div>
    );
  }

  if (sourceError || !sourceData?.source) {
    return (
      <div className="container mx-auto py-6">
        <div className="text-center py-8 text-red-500">
          {sourceError?.message || "Source not found"}
        </div>
        <div className="text-center">
          <Button asChild>
            <Link href="/sources">Back to Sources</Link>
          </Button>
        </div>
      </div>
    );
  }

  const source = sourceData.source;

  return (
    <div className="container mx-auto py-6 space-y-6">
      <div className="flex items-center justify-between">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/sources">
            <ArrowLeft className="h-4 w-4 mr-2" />
            Back to Sources
          </Link>
        </Button>
        
        <div className="flex items-center gap-2">
          <Button
            variant={source.enabled ? "outline" : "default"}
            size="sm"
            onClick={handleToggleSource}
            disabled={toggleSourceMutation.isPending}
          >
            {source.enabled ? (
              <>
                <PowerOff className="h-4 w-4 mr-2" />
                Disable
              </>
            ) : (
              <>
                <Power className="h-4 w-4 mr-2" />
                Enable
              </>
            )}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsEditDialogOpen(true)}
          >
            <Edit className="h-4 w-4 mr-2" />
            Edit
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsDeleteDialogOpen(true)}
          >
            <Trash2 className="h-4 w-4 mr-2" />
            Delete
          </Button>
        </div>
      </div>

      <div>
        <div className="flex items-center gap-4 mb-2">
          <h1 className="text-3xl font-bold tracking-tight text-foreground">{source.name}</h1>
          <Badge className={SOURCE_TYPE_COLORS[source.type]}>
            {SOURCE_TYPE_ICONS[source.type]}
            <span className="ml-1">{SOURCE_TYPE_LABELS[source.type]}</span>
          </Badge>
          <Badge variant={source.enabled ? "default" : "secondary"}>
            {source.enabled ? (
              <>
                <Power className="mr-1 h-3 w-3" />
                Enabled
              </>
            ) : (
              <>
                <PowerOff className="mr-1 h-3 w-3" />
                Disabled
              </>
            )}
          </Badge>
        </div>
        <p className="text-muted-foreground">
          View detailed information and epoch status for this data source
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Actions</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col sm:flex-row gap-4">
            <div className="flex-1">
              <h4 className="font-medium mb-2">Refresh Source Data</h4>
              <p className="text-sm text-muted-foreground mb-4">
                Re-index all epochs for this source to check for new data and update their status.
              </p>
            </div>
            <RefreshSourceButton sourceId={source.id} sourceName={source.name} />
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Source Details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 gap-4">
              <div>
                <label className="text-sm font-medium text-muted-foreground">Source ID</label>
                <p className="text-lg font-mono">{source.id}</p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">Created</label>
                <p className="text-lg">{new Date(source.createdAt).toLocaleString()}</p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">Last Updated</label>
                <p className="text-lg">{new Date(source.updatedAt).toLocaleString()}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Configuration</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 gap-4">
              {formatConfiguration(source.type, source.configuration)}
            </div>
          </CardContent>
        </Card>
      </div>

      {epochsLoading ? (
        <Card>
          <CardContent className="py-8">
            <div className="text-center">Loading epoch data...</div>
          </CardContent>
        </Card>
      ) : epochsError ? (
        <Card>
          <CardContent className="py-8">
            <div className="text-center text-red-500">
              Error loading epochs: {epochsError.message}
            </div>
          </CardContent>
        </Card>
      ) : epochsData ? (
        <>
          <div className="grid gap-6 md:grid-cols-4">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm font-medium">Total Epochs</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{epochsData.statistics.total}</div>
              </CardContent>
            </Card>
            {Object.entries(epochsData.statistics.byStatus).map(([status, count]) => (
              <Card key={status}>
                <CardHeader className="pb-3">
                  <CardTitle className="text-sm font-medium">{status}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{count}</div>
                  <Badge className={`mt-2 ${getStatusColor(status)}`}>
                    {epochsData.statistics.total > 0 
                      ? ((count / epochsData.statistics.total) * 100).toFixed(1) 
                      : 0}%
                  </Badge>
                </CardContent>
              </Card>
            ))}
          </div>

          <Card>
            <CardHeader>
              <CardTitle>Epoch Status</CardTitle>
            </CardHeader>
            <CardContent>
              {epochsData.epochs.length === 0 ? (
                <p className="text-center py-8 text-muted-foreground">
                  No epochs found for this source
                </p>
              ) : (
                <div className="rounded-md border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Epoch</TableHead>
                        <TableHead>Status</TableHead>
                        <TableHead>Available Indexes</TableHead>
                        <TableHead>Last Updated</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {epochsData.epochs.map((epoch) => (
                        <TableRow key={epoch.epochNumber}>
                          <TableCell>
                            <Link 
                              href={`/epochs/${epoch.epochNumber}`}
                              className="font-mono hover:underline"
                            >
                              {epoch.epochNumber}
                            </Link>
                          </TableCell>
                          <TableCell>
                            <Badge className={getStatusColor(epoch.status)}>
                              {epoch.status}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-wrap gap-1">
                              {epoch.indexes.map((index) => (
                                <Badge key={index.type} variant="outline" className="text-xs">
                                  {index.type}
                                </Badge>
                              ))}
                              {epoch.indexes.length === 0 && (
                                <span className="text-muted-foreground text-sm">None</span>
                              )}
                            </div>
                          </TableCell>
                          <TableCell>
                            {epoch.lastUpdated ? new Date(epoch.lastUpdated).toLocaleString() : 'N/A'}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
            </CardContent>
          </Card>
        </>
      ) : null}

      <SourceDialog
        open={isEditDialogOpen}
        onOpenChange={setIsEditDialogOpen}
        source={source}
        onSuccess={() => {
          queryClient.invalidateQueries({ queryKey: ["source", id] });
          queryClient.invalidateQueries({ queryKey: ["sources"] });
          setIsEditDialogOpen(false);
        }}
      />

      <DeleteSourceDialog
        open={isDeleteDialogOpen}
        onOpenChange={setIsDeleteDialogOpen}
        source={source}
        onSuccess={handleDeleteSuccess}
      />
    </div>
  );
}
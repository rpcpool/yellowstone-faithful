"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, Edit, Trash2, Power, PowerOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import { SourceDialog } from "@/components/sources/source-dialog";
import { DeleteSourceDialog } from "@/components/sources/delete-source-dialog";
import { DataSourceType } from "@/generated/prisma";

interface Source {
  id: string;
  name: string;
  type: DataSourceType;
  configuration: Record<string, unknown>;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

interface SourcesResponse {
  sources: Source[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
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

export default function SourcesPage() {
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);
  const [typeFilter, setTypeFilter] = useState<DataSourceType | "all">("all");
  const [searchQuery, setSearchQuery] = useState("");
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [editingSource, setEditingSource] = useState<Source | null>(null);
  const [deletingSource, setDeletingSource] = useState<Source | null>(null);

  const queryClient = useQueryClient();

  const { data, isLoading, error } = useQuery<SourcesResponse>({
    queryKey: ["sources", page, pageSize, typeFilter, searchQuery],
    queryFn: async () => {
      const params = new URLSearchParams({
        page: page.toString(),
        pageSize: pageSize.toString(),
      });

      if (typeFilter !== "all") {
        params.append("type", typeFilter);
      }

      if (searchQuery) {
        params.append("search", searchQuery);
      }

      const response = await fetch(`/api/sources?${params}`);
      if (!response.ok) {
        throw new Error("Failed to fetch sources");
      }
      return response.json();
    },
  });

  const toggleSourceMutation = useMutation({
    mutationFn: async ({ id, enabled }: { id: string; enabled: boolean }) => {
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
      queryClient.invalidateQueries({ queryKey: ["sources"] });
      toast.success("Source updated successfully");
    },
    onError: (error) => {
      toast.error(`Failed to update source: ${error.message}`);
    },
  });

  const handleToggleSource = (source: Source) => {
    toggleSourceMutation.mutate({
      id: source.id,
      enabled: !source.enabled,
    });
  };

  return (
    <div className="container mx-auto py-6 space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-3xl font-bold">Data Sources</h1>
        <Button onClick={() => setIsCreateDialogOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Add Source
        </Button>
      </div>

      <div className="flex gap-4">
        <Input
          placeholder="Search sources..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="max-w-sm"
        />
        <Select
          value={typeFilter}
          onValueChange={(value) => setTypeFilter(value as DataSourceType | "all")}
        >
          <SelectTrigger className="w-[180px]">
            <SelectValue placeholder="Filter by type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Types</SelectItem>
            {Object.entries(SOURCE_TYPE_LABELS).map(([value, label]) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {isLoading ? (
        <div className="text-center py-8">Loading sources...</div>
      ) : error ? (
        <div className="text-center py-8 text-red-500">
          Error loading sources: {error.message}
        </div>
      ) : data?.sources.length === 0 ? (
        <div className="text-center py-8 text-muted-foreground">
          No sources found. Click &quot;Add Source&quot; to create one.
        </div>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data?.sources.map((source) => (
                <TableRow key={source.id}>
                  <TableCell className="font-medium">{source.name}</TableCell>
                  <TableCell>
                    <Badge
                      variant="outline"
                      className={SOURCE_TYPE_COLORS[source.type]}
                    >
                      {SOURCE_TYPE_LABELS[source.type]}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={source.enabled ? "default" : "secondary"}
                      className="cursor-pointer"
                      onClick={() => handleToggleSource(source)}
                    >
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
                  </TableCell>
                  <TableCell>
                    {new Date(source.createdAt).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    {new Date(source.updatedAt).toLocaleDateString()}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setEditingSource(source)}
                    >
                      <Edit className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setDeletingSource(source)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {data && data.totalPages > 1 && (
        <div className="flex justify-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
          >
            Previous
          </Button>
          <span className="py-2 px-4">
            Page {page} of {data.totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage((p) => Math.min(data.totalPages, p + 1))}
            disabled={page === data.totalPages}
          >
            Next
          </Button>
        </div>
      )}

      <SourceDialog
        open={isCreateDialogOpen || !!editingSource}
        onOpenChange={(open) => {
          if (!open) {
            setIsCreateDialogOpen(false);
            setEditingSource(null);
          }
        }}
        source={editingSource}
        onSuccess={() => {
          queryClient.invalidateQueries({ queryKey: ["sources"] });
          setIsCreateDialogOpen(false);
          setEditingSource(null);
        }}
      />

      <DeleteSourceDialog
        open={!!deletingSource}
        onOpenChange={(open) => {
          if (!open) {
            setDeletingSource(null);
          }
        }}
        source={deletingSource}
        onSuccess={() => {
          queryClient.invalidateQueries({ queryKey: ["sources"] });
          setDeletingSource(null);
        }}
      />
    </div>
  );
}
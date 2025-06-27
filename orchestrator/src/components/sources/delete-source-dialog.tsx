"use client";

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
import { Alert, AlertDescription } from "@/components/ui/alert";
import { AlertTriangle } from "lucide-react";
import { toast } from "sonner";

interface Source {
  id: string;
  name: string;
  type: string;
}

interface DeleteSourceDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  source: Source | null;
  onSuccess: () => void;
}

export function DeleteSourceDialog({
  open,
  onOpenChange,
  source,
  onSuccess,
}: DeleteSourceDialogProps) {
  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      const response = await fetch(`/api/sources/${id}`, {
        method: "DELETE",
      });
      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || "Failed to delete source");
      }
      return response.json();
    },
    onSuccess: () => {
      toast.success("Source deleted successfully");
      onSuccess();
    },
    onError: (error) => {
      toast.error(`Failed to delete source: ${error.message}`);
    },
  });

  const handleDelete = () => {
    if (source) {
      deleteMutation.mutate(source.id);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete Source</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete this source?
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <Alert variant="destructive">
            <AlertTriangle className="h-4 w-4" />
            <AlertDescription>
              This action cannot be undone. The source &quot;{source?.name}&quot; will be
              permanently deleted.
            </AlertDescription>
          </Alert>

          <div className="rounded-lg border p-4">
            <dl className="space-y-2">
              <div>
                <dt className="text-sm font-medium text-muted-foreground">
                  Name
                </dt>
                <dd className="text-sm">{source?.name}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-muted-foreground">
                  Type
                </dt>
                <dd className="text-sm">{source?.type}</dd>
              </div>
            </dl>
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={deleteMutation.isPending}
          >
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={deleteMutation.isPending}
          >
            {deleteMutation.isPending ? "Deleting..." : "Delete"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
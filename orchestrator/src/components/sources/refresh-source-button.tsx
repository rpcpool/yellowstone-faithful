'use client';

import { Button } from '@/components/ui/button';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2, RefreshCw } from 'lucide-react';
import { useState } from 'react';
import { toast } from 'sonner';

interface Props {
  sourceId: string;
  sourceName: string;
}

export function RefreshSourceButton({ sourceId, sourceName }: Props) {
  const [isScheduling, setIsScheduling] = useState(false);
  const [lastJobId, setLastJobId] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: async () => {
      const response = await fetch('/api/jobs/schedule', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          jobType: 'refreshAllEpochs',
          params: {
            sourceId,
            batchSize: 100
          }
        })
      });
      
      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.details || error.error || 'Failed to schedule job');
      }
      
      return response.json();
    },
    onMutate: () => {
      setIsScheduling(true);
    },
    onSuccess: (data) => {
      toast.success(`Refresh job scheduled for ${sourceName}`);
      setLastJobId(data.jobId);
      // Invalidate relevant queries
      queryClient.invalidateQueries({ queryKey: ['source-epochs', sourceId] });
    },
    onError: (error: Error) => {
      toast.error(`Failed to schedule refresh: ${error.message}`);
    },
    onSettled: () => {
      setIsScheduling(false);
    },
  });

  return (
    <div className="flex flex-col gap-4">
      <Button
        onClick={() => mutation.mutate()}
        disabled={isScheduling}
        size="default"
        variant="outline"
      >
        {isScheduling ? (
          <>
            <Loader2 className="animate-spin -ml-1 mr-2 h-4 w-4" />
            Scheduling...
          </>
        ) : (
          <>
            <RefreshCw className="mr-2 h-4 w-4" />
            Refresh All Epochs
          </>
        )}
      </Button>
      
      {lastJobId && (
        <div className="text-sm text-muted-foreground">
          <p>Job scheduled with ID: <code className="text-xs">{lastJobId}</code></p>
          <p className="text-xs mt-1">Check the Jobs tab in Settings for progress.</p>
        </div>
      )}
    </div>
  );
}
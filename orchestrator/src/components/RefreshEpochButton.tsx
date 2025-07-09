'use client';

import { Button } from '@/components/ui/button';
import { getRefreshJobStatus, scheduleTask } from '@/lib/epochs/actions';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle, Clock, Loader2, RefreshCw, XCircle } from 'lucide-react';
import { useTransition } from 'react';
import { toast } from 'sonner';

interface Props {
  epochId: number;
}

export function RefreshEpochButton({ epochId }: Props) {
  const [isPending, startTransition] = useTransition();
  const qc = useQueryClient();

  const { data: jobs = [], isLoading } = useQuery({
    queryKey: ['refreshEpochJobs', epochId],
    queryFn: () => getRefreshJobStatus(epochId).then(r => r.jobs),
    refetchInterval: 10_000,
  });

  const mutation = useMutation({
    mutationFn: () => scheduleTask('refreshEpoch', { epochId }),
    onSuccess: async res => {
      if (res.success) {
        toast.success(res.message ?? 'Refresh job scheduled!');
        await qc.invalidateQueries({ queryKey: ['refreshEpochJobs', epochId] });
      } else {
        toast.error(res.message ?? 'Failed to schedule job');
      }
    },
    onError: () => toast.error('Unexpected error while scheduling job'),
  });

  const latest = jobs[0];
  const active = latest && (latest.status === 'queued' || latest.status === 'processing');

  const handleClick = () => {
    startTransition(() => mutation.mutate());
  };

  const icon = {
    queued:  <Clock    className="h-4 w-4 text-blue-500 dark:text-blue-400" />,
    processing: <Loader2  className="h-4 w-4 text-yellow-500 dark:text-yellow-400 animate-spin" />,
    completed:  <CheckCircle className="h-4 w-4 text-green-500 dark:text-green-400" />,
    failed:     <XCircle  className="h-4 w-4 text-red-500 dark:text-red-400" />,
  };

  const label = {
    queued: 'Queued',
    processing: 'Processing',
    completed: 'Completed',
    failed: 'Failed',
  };

  return (
    <Button
      onClick={handleClick}
      disabled={isPending || active || isLoading}
      size="lg"
      className="w-full sm:w-auto bg-green-600 hover:bg-green-700 dark:bg-green-700 dark:hover:bg-green-800 text-white font-semibold px-6 py-3 shadow-md hover:shadow-lg transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
    >
      {isPending ? (
        <>
          <Loader2 className="animate-spin -ml-1 mr-2 h-5 w-5" />
          Scheduling…
        </>
      ) : active ? (
        <>
          {icon[latest!.status]}
          <span className="ml-2">Job {label[latest!.status]}</span>
        </>
      ) : (
        <>
          <RefreshCw className="mr-2 h-5 w-5" />
          Refresh Data
        </>
      )}
    </Button>
  );
}
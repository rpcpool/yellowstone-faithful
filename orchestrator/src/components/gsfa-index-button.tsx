'use client';

import { Button } from "@/components/ui/button";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle, Clock, Download, Loader2, XCircle } from "lucide-react";
import { useTransition } from "react";
import { toast } from "sonner";
import { getGSFAJobStatus, scheduleTask } from "../lib/epochs/actions";

interface GSFAIndexButtonProps {
  epochId: number;
}


export function GSFAIndexButton({ epochId }: GSFAIndexButtonProps) {
  const [isPending, startTransition] = useTransition();
  const queryClient = useQueryClient();

  const {
    data: jobs = [],
    isLoading: isLoadingJobs,
  } = useQuery({
    queryKey: ['gsfaJobs', epochId],
    queryFn: () => getGSFAJobStatus(epochId).then((res) => res.jobs),
    refetchInterval: 10000,
  });

  const mutation = useMutation({
    mutationFn: () => scheduleTask('getGsfaIndex', { epochId }),
    onSuccess: async (result) => {
      if (result.success) {
        toast.success(result.message || 'GSFA index job scheduled successfully!');
        await queryClient.invalidateQueries({ queryKey: ['gsfaJobs', epochId] });
      } else {
        toast.error(result.message || 'Failed to schedule GSFA index job');
      }
    },
    onError: () => {
      toast.error('An unexpected error occurred while scheduling the job');
    },
  });

  const latestJob = jobs.length > 0 ? jobs[0] : null;
  const hasActiveJob = latestJob && (latestJob.status === 'queued' || latestJob.status === 'processing');

  const handleScheduleGSFA = () => {
    startTransition(() => {
      mutation.mutate();
    });
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'queued':
        return <Clock className="h-4 w-4 text-blue-500 dark:text-blue-400" />;
      case 'processing':
        return <Loader2 className="h-4 w-4 text-yellow-500 dark:text-yellow-400 animate-spin" />;
      case 'completed':
        return <CheckCircle className="h-4 w-4 text-green-500 dark:text-green-400" />;
      case 'failed':
        return <XCircle className="h-4 w-4 text-red-500 dark:text-red-400" />;
      default:
        return <Loader2 className="h-4 w-4 text-yellow-500 dark:text-yellow-400 animate-spin" />;;
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'queued':
        return 'Queued';
      case 'processing':
        return 'Processing';
      case 'completed':
        return 'Completed';
      case 'failed':
        return 'Failed';
      default:
        return status;
    }
  };

  return (
    <Button
      onClick={handleScheduleGSFA}
      disabled={isPending || hasActiveJob || isLoadingJobs}
      size="lg"
      className="w-full sm:w-auto bg-blue-600 hover:bg-blue-700 dark:bg-blue-700 dark:hover:bg-blue-800 text-white font-semibold px-6 py-3 shadow-md hover:shadow-lg transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
    >
      {isPending ? (
        <>
          <Loader2 className="animate-spin -ml-1 mr-2 h-5 w-5" />
          Downloading Index...
        </>
      ) : hasActiveJob ? (
        <>
          {getStatusIcon(latestJob!.status)}
          <span className="ml-2">Job {getStatusText(latestJob!.status)}</span>
        </>
      ) : (
        <>
          <Download className="mr-2 h-5 w-5" />
          Download Index
        </>
      )}
    </Button>
  );
} 
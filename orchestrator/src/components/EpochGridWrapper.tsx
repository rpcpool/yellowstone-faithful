"use client";

import { EpochGrid } from "@/components/EpochGrid";
import { EpochGridSkeleton } from "@/components/skeletons";
import { useQuery } from "@tanstack/react-query";

interface EpochGridWrapperProps {
  onEpochClick: (epochIndex: number) => void;
  totalEpochs?: number;
}

export function EpochGridWrapper({ onEpochClick, totalEpochs = 792 }: EpochGridWrapperProps) {
  const {
    data: epochs = Array(totalEpochs).fill(null),
    isLoading,
    error
  } = useQuery({
    queryKey: ['epochs', totalEpochs],
    queryFn: async () => {
      const res = await fetch(`/api/epochs?start=0&end=${totalEpochs}`);
      if (!res.ok) throw new Error('Failed to fetch epochs');
      const data = await res.json();
      return data.epochs && Array.isArray(data.epochs)
        ? data.epochs
        : Array(totalEpochs).fill(null);
    },
    initialData: Array(totalEpochs).fill(null),
    staleTime: 60000, // Consider data fresh for 1 minute
    gcTime: 10 * 60 * 1000, // Keep in cache for 10 minutes
  });

  if (isLoading) {
    return <EpochGridSkeleton />;
  }

  if (error) {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="text-center">
          <p className="text-muted-foreground text-red-500 mb-4">
            Failed to load epoch data. Please try refreshing the page.
          </p>
          <button 
            onClick={() => window.location.reload()} 
            className="px-4 py-2 bg-primary text-primary-foreground rounded hover:bg-primary/90"
          >
            Refresh Page
          </button>
        </div>
      </div>
    );
  }

  return (
    <EpochGrid 
      epochs={epochs}
      totalEpochs={totalEpochs}
      onEpochClick={onEpochClick}
    />
  );
} 
"use client";

import { DashboardStats } from "@/components/DashboardStats";
import { EpochDetailsDialog } from "@/components/EpochDetailsDialog";
import { EpochGrid } from "@/components/EpochGrid";
import { DashboardStatsSkeleton, EpochGridSkeleton } from "@/components/skeletons";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

const TOTAL_EPOCHS = 792;

type EpochDetails = {
  epoch: {
    id: number;
    epoch: string;
    status: string;
    createdAt: string;
    updatedAt: string;
  };
  indexes: Array<{
    id: number;
    epoch: string;
    type: string;
    size: string;
    status: string;
    location: string;
    createdAt: string;
    updatedAt: string;
  }>;
  gsfa: {
    id: number;
    epoch: string;
    exists: boolean;
    location: string;
    createdAt: string;
    updatedAt: string;
  } | null;
  stats: {
    totalIndexes: number;
    totalSize: number;
    statusCounts: Record<string, number>;
    typeCounts: Record<string, number>;
  };
};

export default function Home() {
  const [selectedEpoch, setSelectedEpoch] = useState<number | null>(null);
  const [isDialogOpen, setIsDialogOpen] = useState(false);

  const {
    data: stats,
    isLoading: isLoadingStats,
  } = useQuery({
    queryKey: ['stats'],
    queryFn: async () => {
      const res = await fetch('/api/stats');
      if (!res.ok) throw new Error('Failed to fetch stats');
      return res.json();
    },
    staleTime: 30000, // Consider data fresh for 30 seconds
    gcTime: 5 * 60 * 1000, // Keep in cache for 5 minutes
  });

  const {
    data: epochs = Array(TOTAL_EPOCHS).fill(null),
    isLoading: isLoadingEpochs,
  } = useQuery({
    queryKey: ['epochs', TOTAL_EPOCHS],
    queryFn: async () => {
      console.log('Fetching epochs...');
      const res = await fetch(`/api/epochs?start=0&end=${TOTAL_EPOCHS}`);
      if (!res.ok) throw new Error('Failed to fetch epochs');
      const data = await res.json();
      return data.epochs && Array.isArray(data.epochs)
        ? data.epochs
        : Array(TOTAL_EPOCHS).fill(null);
    },
    staleTime: 60000, // Consider data fresh for 1 minute
    gcTime: 10 * 60 * 1000, // Keep in cache for 10 minutes
  });

  const {
    data: epochDetails,
    isFetching: isLoadingDetails,
    refetch: refetchEpochDetails,
  } = useQuery<EpochDetails | null>({
    queryKey: ['epoch', selectedEpoch],
    queryFn: async () => {
      if (selectedEpoch === null) return null;
      const res = await fetch(`/api/epochs/${selectedEpoch}`);
      if (!res.ok) throw new Error('Failed to fetch epoch details');
      return res.json();
    },
    enabled: selectedEpoch !== null,
    staleTime: 30000, // Consider data fresh for 30 seconds
    gcTime: 5 * 60 * 1000, // Keep in cache for 5 minutes
  });

  const handleEpochClick = (epochIndex: number) => {
    setSelectedEpoch(epochIndex);
    setIsDialogOpen(true);
  };


  return (
    <div className=" px-4 py-8 w-full flex flex-col gap-4">
      {/* Dashboard Stats with skeleton loading */}
      {isLoadingStats ? (
        <DashboardStatsSkeleton />
      ) : (
        <DashboardStats 
          stats={stats}
        />
      )}

      {/* Epoch Grid with skeleton loading */}
        {isLoadingEpochs ? (
          <EpochGridSkeleton />
        ) : (
          <EpochGrid 
            epochs={epochs}
            totalEpochs={TOTAL_EPOCHS}
            onEpochClick={handleEpochClick}
          />
        )}

      {/* Epoch Details Dialog */}
      <EpochDetailsDialog
        isOpen={isDialogOpen}
        onOpenChange={setIsDialogOpen}
        selectedEpoch={selectedEpoch}
        epochDetails={epochDetails ?? null}
        isLoadingDetails={isLoadingDetails}
        onRetry={() => refetchEpochDetails()}
      />
    </div>
  );
}

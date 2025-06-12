"use client";

import { DashboardStats } from "@/components/DashboardStats";
import { DashboardStatsSkeleton } from "@/components/skeletons";
import { useQuery } from "@tanstack/react-query";

interface DashboardStatsWrapperProps {
  statusCounts: Record<string, number>;
}

export function DashboardStatsWrapper({ statusCounts }: DashboardStatsWrapperProps) {
  const {
    data: stats,
    isLoading,
    error
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

  if (isLoading) {
    return <DashboardStatsSkeleton />;
  }

  if (error) {
    return (
      <div className="flex flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
        <div className="flex flex-col gap-2">
          <h1 className="text-3xl font-bold tracking-tight">Epochs Dashboard</h1>
          <p className="text-muted-foreground text-red-500">
            Failed to load dashboard stats. Please try refreshing the page.
          </p>
        </div>
      </div>
    );
  }

  return (
    <DashboardStats 
      stats={stats}
      statusCounts={statusCounts}
    />
  );
} 
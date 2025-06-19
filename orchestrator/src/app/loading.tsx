import { DashboardStatsSkeleton, EpochGridSkeleton } from "@/components/skeletons";

export default function Loading() {
  return (
    <div className="w-full flex flex-col">
      <DashboardStatsSkeleton />
      <EpochGridSkeleton />
    </div>
  );
} 
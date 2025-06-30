import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

interface EpochsStatusOverviewCardProps {
  epochsData: { totalCount: number } | undefined;
  epochStats: Record<string, number>;
  currentEpoch: number | null;
}

export function EpochsStatusOverviewCard({ epochsData, epochStats, currentEpoch }: EpochsStatusOverviewCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Current Status</CardTitle>
        <CardDescription>Overview of epochs in the database</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="text-center">
            <p className="text-2xl font-bold">{epochsData?.totalCount || 0}</p>
            <p className="text-sm text-muted-foreground">Total Epochs</p>
          </div>
          <div className="text-center">
            <p className="text-2xl font-bold">{epochStats.Complete || 0}</p>
            <p className="text-sm text-muted-foreground">Complete</p>
          </div>
          <div className="text-center">
            <p className="text-2xl font-bold">{epochStats.Processing || 0}</p>
            <p className="text-sm text-muted-foreground">Processing</p>
          </div>
          <div className="text-center">
            <p className="text-2xl font-bold">{epochStats.NotProcessed || 0}</p>
            <p className="text-sm text-muted-foreground">Not Processed</p>
          </div>
        </div>
        {currentEpoch !== null && (
          <div className="mt-4 text-sm text-muted-foreground text-center">
            Current Solana epoch: {currentEpoch}
          </div>
        )}
      </CardContent>
    </Card>
  );
} 
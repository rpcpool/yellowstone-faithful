import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

interface EpochData {
  id: number;
  status: string;
  createdAt: string;
  updatedAt: string;
}

type BadgeVariant = "default" | "destructive" | "outline" | "secondary" | undefined;

interface RecentEpochsCardProps {
  isLoadingEpochs: boolean;
  existingEpochs: EpochData[];
  getStatusBadgeVariant: (status: string) => BadgeVariant;
}

export function RecentEpochsCard({ isLoadingEpochs, existingEpochs, getStatusBadgeVariant }: RecentEpochsCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent Epochs</CardTitle>
        <CardDescription>Last 10 epochs in the database</CardDescription>
      </CardHeader>
      <CardContent>
        {isLoadingEpochs ? (
          <p className="text-sm text-muted-foreground">Loading...</p>
        ) : existingEpochs.length === 0 ? (
          <p className="text-sm text-muted-foreground">No epochs in database</p>
        ) : (
          <div className="space-y-2">
            {existingEpochs.slice(0, 10).map((epoch: EpochData) => (
              <div key={epoch.id} className="flex items-center justify-between py-2 border-b">
                <div className="flex items-center gap-4">
                  <span className="font-mono">Epoch {epoch.id}</span>
                  <Badge variant={getStatusBadgeVariant(epoch.status)}>
                    {epoch.status}
                  </Badge>
                </div>
                <span className="text-sm text-muted-foreground">
                  Added {new Date(epoch.createdAt).toLocaleDateString()}
                </span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
} 
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

interface AddAllEpochsCardProps {
  currentEpoch: number | null;
  handleAddAllToDate: () => void;
  isPending: boolean;
}

export function AddAllEpochsCard({ currentEpoch, handleAddAllToDate, isPending }: AddAllEpochsCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Add All Epochs</CardTitle>
        <CardDescription>Add all epochs from 0 to current</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm">
              This will add all epochs from 0 to {currentEpoch ?? "..."} (current epoch)
            </p>
            <p className="text-sm text-muted-foreground mt-1">
              Note: This may take a while for large ranges
            </p>
          </div>
          <Button
            onClick={handleAddAllToDate}
            disabled={currentEpoch === null || isPending}
            variant="secondary"
          >
            Add All Epochs
          </Button>
        </div>
      </CardContent>
    </Card>
  );
} 
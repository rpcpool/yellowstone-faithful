import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface AddSingleEpochCardProps {
  singleEpoch: string;
  setSingleEpoch: (value: string) => void;
  handleAddSingleEpoch: () => void;
  isPending: boolean;
}

export function AddSingleEpochCard({ singleEpoch, setSingleEpoch, handleAddSingleEpoch, isPending }: AddSingleEpochCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Add Single Epoch</CardTitle>
        <CardDescription>Add a specific epoch to monitor</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-end gap-4">
          <div className="flex-1 flex flex-col gap-1">
            <Label htmlFor="single-epoch">Epoch Number</Label>
            <Input
              id="single-epoch"
              type="number"
              placeholder="e.g., 500"
              value={singleEpoch}
              onChange={(e) => setSingleEpoch(e.target.value)}
              min="0"
            />
          </div>
          <Button
            onClick={handleAddSingleEpoch}
            disabled={!singleEpoch || isPending}
          >
            Add Epoch
          </Button>
        </div>
      </CardContent>
    </Card>
  );
} 
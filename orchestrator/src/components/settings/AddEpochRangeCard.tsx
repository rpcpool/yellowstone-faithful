import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface AddEpochRangeCardProps {
  rangeStart: string;
  setRangeStart: (value: string) => void;
  rangeEnd: string;
  setRangeEnd: (value: string) => void;
  handleAddRange: () => void;
  isPending: boolean;
}

export function AddEpochRangeCard({ rangeStart, setRangeStart, rangeEnd, setRangeEnd, handleAddRange, isPending }: AddEpochRangeCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Add Epoch Range</CardTitle>
        <CardDescription>Add multiple epochs by specifying a range</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-end gap-4">
          <div className="flex-1 flex flex-col gap-1">
            <Label htmlFor="range-start">Start Epoch</Label>
            <Input
              id="range-start"
              type="number"
              placeholder="e.g., 400"
              value={rangeStart}
              onChange={(e) => setRangeStart(e.target.value)}
              min="0"
            />
          </div>
          <div className="flex-1 flex flex-col gap-1">
            <Label htmlFor="range-end">End Epoch</Label>
            <Input
              id="range-end"
              type="number"
              placeholder="e.g., 500"
              value={rangeEnd}
              onChange={(e) => setRangeEnd(e.target.value)}
              min="0"
            />
          </div>
          <Button
            onClick={handleAddRange}
            disabled={!rangeStart || !rangeEnd || isPending}
          >
            Add Range
          </Button>
        </div>
        <p className="text-sm text-muted-foreground mt-2">
          Maximum 1000 epochs per range
        </p>
      </CardContent>
    </Card>
  );
} 
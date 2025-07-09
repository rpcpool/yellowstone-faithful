import { Alert, AlertDescription } from "@/components/ui/alert";

interface EpochsResultAlertProps {
  addResult: { success: boolean; message: string } | null;
}

export function EpochsResultAlert({ addResult }: EpochsResultAlertProps) {
  if (!addResult) return null;
  return (
    <Alert variant={addResult.success ? "default" : "destructive"}>
      <AlertDescription>{addResult.message}</AlertDescription>
    </Alert>
  );
} 
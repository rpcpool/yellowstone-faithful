export interface Task {
  name: string;
  description: string;
  args: Record<string, unknown>;
  requiredSource?: string;
  requiredArgs?: string[];
  requiredWorker?: string;

  run: (args: Record<string, unknown>) => Promise<boolean>;
  validateArgs: (args: Record<string, unknown>) => boolean;
  schedule: (args: Record<string, unknown>) => Promise<string>;
}
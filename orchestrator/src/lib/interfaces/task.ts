export interface Task<TArgs = unknown> {
  name: string;
  description: string;
  args: unknown; // Can be a ZodObject or other schema
  requiredSource?: string;
  requiredArgs?: string[];
  requiredWorker?: string;

  run: (args: TArgs) => Promise<boolean>;
  validateArgs: (args: unknown) => boolean;
  schedule: (args: TArgs) => Promise<string>;
}
// Refresh a single source

import { checkSource } from "@/lib/epochs";
import { getDataSource } from "@/lib/epochs/data-sources";
import { updateEpochStatus } from "@/lib/epochs/update-epoch-status";
import { client } from "@/lib/infrastructure/faktory/faktory-client";
import type { Job } from "faktory-worker";
import { Task } from "@/lib/interfaces/task";
import { z } from "zod";

export const refreshSourceArgsSchema = z.object({
  epochId: z.number(),
  sourceName: z.string(),
});

type RefreshSourceArgs = z.infer<typeof refreshSourceArgsSchema>;

export const refreshSourceTask: Task<RefreshSourceArgs> = {
  name: "refreshSource",
  description: "Refreshes a single data source for a given epoch and updates its status.",
  args: refreshSourceArgsSchema,
  validateArgs: (args: unknown): boolean => {
    return refreshSourceArgsSchema.safeParse(args).success;
  },
  run: async (args: RefreshSourceArgs): Promise<boolean> => {
    const { epochId, sourceName } = args;
    const source = getDataSource(sourceName);
    await checkSource(epochId, source);
    await updateEpochStatus(epochId);
    return true;
  },
  schedule: async (args: RefreshSourceArgs): Promise<string> => {
    const job: Job = client.job(refreshSourceTask.name, args);
    job.queue = "default";
    await job.push();
    return job.jid;
  },
};
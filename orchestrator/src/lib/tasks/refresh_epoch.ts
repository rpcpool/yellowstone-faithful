// Refresh a single source

import { checkSource } from "@/lib/epochs";
import { updateEpochStatus } from "@/lib/epochs/update-epoch-status";
import { client } from "@/lib/infrastructure/faktory/faktory-client";
import type { Job } from "faktory-worker";
import { Task } from "@/lib/interfaces/task";
import { z } from "zod";
import { dataSources } from "../epochs/data-sources";

export const refreshEpochArgsSchema = z.object({
  epochId: z.number(),
});

type RefreshEpochArgs = z.infer<typeof refreshEpochArgsSchema>;

export const refreshEpochTask: Task<RefreshEpochArgs> = {
  name: "refreshEpoch",
  description: "Refreshes a single epoch and updates its status.",
  args: refreshEpochArgsSchema,
  validateArgs: (args: unknown): boolean => {
    return refreshEpochArgsSchema.safeParse(args).success;
  },
  run: async (args: RefreshEpochArgs): Promise<boolean> => {
    const { epochId } = args;
    for (const source of dataSources) {
      await checkSource(epochId, source);
    }
    await updateEpochStatus(epochId);
    return true;
  },
  schedule: async (args: RefreshEpochArgs): Promise<string> => {
    const job: Job = client.job(refreshEpochTask.name, args);
    job.queue = "default";
    await job.push();
    return job.jid;
  },
};
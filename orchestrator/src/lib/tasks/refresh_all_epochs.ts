// Refresh all epochs by scheduling individual refreshEpoch jobs

import { client } from "@/lib/infrastructure/faktory/faktory-client";
import { prisma } from "@/lib/infrastructure/persistence/prisma";
import type { Job } from "faktory-worker";
import { Task } from "@/lib/interfaces/task";
import { z } from "zod";
import { refreshEpochTask } from "./refresh_epoch";
import { refreshSourceTask } from "./refresh_source";

export const refreshAllEpochsArgsSchema = z.object({
  sourceName: z.string().optional(), // Optional: filter by specific source
  batchSize: z.number().optional().default(100), // Number of epochs to process at once
});

type RefreshAllEpochsArgs = z.infer<typeof refreshAllEpochsArgsSchema>;

export const refreshAllEpochsTask: Task<RefreshAllEpochsArgs> = {
  name: "refreshAllEpochs",
  description: "Schedules refresh jobs for all epochs, optionally filtered by source or status.",
  args: refreshAllEpochsArgsSchema,
  validateArgs: (args: unknown): boolean => {
    return refreshAllEpochsArgsSchema.safeParse(args).success;
  },
  run: async (args: RefreshAllEpochsArgs): Promise<boolean> => {
    const { sourceName, batchSize = 100 } = args;
    
    let scheduledCount = 0;
    let failedCount = 0;
    let offset = 0;
    
    console.log(`Starting refresh for all epochs with params:`, { sourceName, batchSize });
    
    while (true) {
      // Fetch epochs in batches
      const epochs = await prisma.epoch.findMany({
        skip: offset,
        take: batchSize,
        orderBy: { id: 'asc' }
      });
      
      if (epochs.length === 0) {
        break; // No more epochs to process
      }
      
      // Schedule jobs for each epoch
      for (const epoch of epochs) {
        try {
          if (sourceName) {
            // Schedule refreshSource job for specific source
            await refreshSourceTask.schedule({
              epochId: epoch.id,
              sourceName: sourceName
            });
          } else {
            // Schedule refreshEpoch job for all sources
            await refreshEpochTask.schedule({
              epochId: epoch.id
            });
          }
          scheduledCount++;
        } catch (error) {
          console.error(`Failed to schedule refresh for epoch ${epoch.id}:`, error);
          failedCount++;
        }
      }
      
      offset += batchSize;
      
      // Add a small delay to avoid overwhelming the queue
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    
    console.log(`Refresh all epochs completed. Scheduled: ${scheduledCount}, Failed: ${failedCount}`);
    
    return failedCount === 0;
  },
  schedule: async (args: RefreshAllEpochsArgs): Promise<string> => {
    const job: Job = client.job(refreshAllEpochsTask.name, args);
    job.queue = "default";
    job.args = [args]; // Ensure args are properly formatted
    await job.push();
    return job.jid;
  },
};
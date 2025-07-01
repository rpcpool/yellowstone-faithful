// Refresh all sources for a single epoch

import { client } from "@/lib/infrastructure/faktory/faktory-client";
import type { Job } from "faktory-worker";
import { Task } from "@/lib/interfaces/task";
import { z } from "zod";
import { getDataSourcesFromDB } from "../epochs/data-sources-db";
import { refreshSourceTask } from "./refresh_source";

export const refreshEpochArgsSchema = z.object({
  epochId: z.number(),
});

type RefreshEpochArgs = z.infer<typeof refreshEpochArgsSchema>;

export const refreshEpochTask: Task<RefreshEpochArgs> = {
  name: "refreshEpoch",
  description: "Schedules refresh jobs for all sources for a single epoch.",
  args: refreshEpochArgsSchema,
  validateArgs: (args: unknown): boolean => {
    return refreshEpochArgsSchema.safeParse(args).success;
  },
  run: async (args: RefreshEpochArgs): Promise<boolean> => {
    const { epochId } = args;
    const dataSources = await getDataSourcesFromDB();
    
    console.log(`[refreshEpoch] Scheduling refresh for epoch ${epochId} across ${dataSources.length} sources`);
    
    // Schedule individual refreshSource jobs for each source
    const schedulePromises = dataSources.map(async (source) => {
      try {
        const jobId = await refreshSourceTask.schedule({
          epochId,
          sourceId: source.id!
        });
        console.log(`[refreshEpoch] Scheduled refreshSource job ${jobId} for source ${source.name}`);
        return { success: true, source: source.name, jobId };
      } catch (error) {
        console.error(`[refreshEpoch] Failed to schedule refresh for source ${source.name}:`, error);
        return { success: false, source: source.name, error };
      }
    });
    
    // Wait for all jobs to be scheduled
    const results = await Promise.all(schedulePromises);
    
    // Count successful schedules
    const successCount = results.filter(r => r.success).length;
    console.log(`[refreshEpoch] Successfully scheduled ${successCount}/${dataSources.length} refresh jobs for epoch ${epochId}`);
    
    // Note: We don't update epoch status here anymore since each source will update it
    // after checking. The final status will be determined by aggregating all source results.
    
    return successCount === dataSources.length;
  },
  schedule: async (args: RefreshEpochArgs): Promise<string> => {
    const job: Job = client.job(refreshEpochTask.name, args);
    job.queue = "default";
    await job.push();
    return job.jid;
  },
};
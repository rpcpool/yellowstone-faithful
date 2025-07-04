// Refresh a single source

import { checkSource } from "@/lib/epochs";
import { updateEpochStatus } from "@/lib/epochs/update-epoch-status";
import { client } from "@/lib/infrastructure/faktory/faktory-client";
import type { Job } from "faktory-worker";
import { Task } from "@/lib/interfaces/task";
import { z } from "zod";
import { PrismaSourceRepository } from "@/lib/infrastructure/repositories/prisma-source-repository";
import { getQueueForSource } from "@/lib/utils/queue-utils";
import { SourceFactory } from "@/lib/infrastructure/data-sources/source-factory";

export const refreshSourceArgsSchema = z.object({
  epochId: z.number(),
  sourceId: z.string(),
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
    const { epochId, sourceId } = args;
    
    // Get the source from database
    const sourceRepository = new PrismaSourceRepository();
    const sourceEntity = await sourceRepository.findById(sourceId);
    
    if (!sourceEntity) {
      throw new Error(`Source with ID ${sourceId} not found`);
    }
    
    // Create DataSource from the entity
    const dataSource = SourceFactory.createDataSource(sourceEntity);
    
    await checkSource(epochId, dataSource);
    await updateEpochStatus(epochId);
    return true;
  },
  schedule: async (args: RefreshSourceArgs): Promise<string> => {
    // Get the source to determine the appropriate queue
    const sourceRepository = new PrismaSourceRepository();
    const source = await sourceRepository.findById(args.sourceId);
    
    if (!source) {
      throw new Error(`Source with ID ${args.sourceId} not found. Cannot schedule task.`);
    }
    
    // Determine the appropriate queue based on source configuration
    const queue = getQueueForSource(source);
    
    const job: Job = client.job(refreshSourceTask.name, args);
    job.queue = queue;
    await job.push();
    
    console.log(`Scheduled refreshSource for epoch ${args.epochId}, source ${source.name} (ID: ${args.sourceId}) to queue: ${queue}`);
    
    return job.jid;
  },
};
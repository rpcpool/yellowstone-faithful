import { getEpochs, getLatestEpoch } from "@/lib/epochs";
import { client } from "@/lib/infrastructure/faktory/faktory-client";
import { Task } from "@/lib/interfaces/task";
import { prisma } from "@/lib/infrastructure/persistence/prisma";
import { z } from "zod";

export const fetchEpochCidsArgsSchema = z.object({});
type FetchEpochCidsArgs = z.infer<typeof fetchEpochCidsArgsSchema>;

const fetchEpochCidsTask: Task<FetchEpochCidsArgs> = {

  name: "fetchEpochCids",
  description: "Fetches CIDs for all epochs and updates the database.",
  args: fetchEpochCidsArgsSchema,

  validateArgs: (args: unknown): boolean => {
    return fetchEpochCidsArgsSchema.safeParse(args).success;
  },

  async run(): Promise<boolean> {
    try {
      const latestEpoch = await getLatestEpoch();
      const epochs = await getEpochs(0, latestEpoch);
      for (const epoch of epochs) {
        const cid = await fetch(`https://files.old-faithful.net/${epoch.epoch}/epoch-${epoch.epoch}.cid`);
        if (!cid.ok) {
          console.error(`Failed to fetch CID for epoch ${epoch.epoch}`);
          continue;
        }
        const cidText = (await cid.text()).trim();
        await prisma.epoch.update({
          where: { id: epoch.id },
          data: { cid: cidText }
        });
        console.log(`Fetched CID for epoch ${epoch.epoch}: ${cidText}`);
      }
      return true;
    } catch (error) {
      console.error("Error in fetchEpochCidsTask:", error);
      return false;
    }
  },

  schedule: async (args: FetchEpochCidsArgs): Promise<string> => {
    const job = client.job(fetchEpochCidsTask.name, args);
    job.queue = "default";
    job.reserveFor = 1000;
    await job.push();
    return job.jid;
  },
};

export default fetchEpochCidsTask;
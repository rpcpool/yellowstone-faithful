// Refresh a single source

import { FileSystemSource } from "@/lib/data-sources/filesystem-source";
import { getDataSource } from "@/lib/epochs/data-sources";
import { Task } from "@/lib/interfaces/task";

import { client, Job } from "@/lib/faktory";
import { createWriteStream, promises as fs } from "fs";
import path from "path";
import { pipeline } from "stream/promises";
import { z } from "zod";
import { checkSource } from "../epochs/check-source";
import { updateEpochStatus } from "../epochs/update-epoch-status";

// Base URL where all index files are hosted
const BASE_URL = "https://files.old-faithful.net";

// List of index filenames (except CID and epoch) that need the dynamic pieces interpolated
const INDEX_FILE_PATTERNS = [
  "epoch-__EPOCH__-__CID__-mainnet-cid-to-offset-and-size.index",
  "epoch-__EPOCH__-__CID__-mainnet-sig-to-cid.index",
  "epoch-__EPOCH__-__CID__-mainnet-sig-exists.index",
  "epoch-__EPOCH__-__CID__-mainnet-slot-to-cid.index",
  "epoch-__EPOCH__-__CID__-mainnet-slot-to-blocktime.index",
];

export const getStandardIndexesArgsSchema = z.object({
  epochId: z.number(),
  force: z.boolean().optional().default(false),
});

type GetStandardIndexesArgs = z.infer<typeof getStandardIndexesArgsSchema>;

export const getStandardIndexesTask: Task = {
  name: "getStandardIndexes",
  description: "Downloads and stores standard index files for a given epoch.",
  args: getStandardIndexesArgsSchema,
  requiredSource: "Local",
  validateArgs: (args: unknown): boolean => {
    return getStandardIndexesArgsSchema.safeParse(args).success;
  },
  run: async (args: GetStandardIndexesArgs): Promise<boolean> => {
    const { epochId, force } = args;
    const source = getDataSource("Local") as FileSystemSource;

    // Ensure the target directory exists
    const destDir = source.getEpochFilePath(epochId);
    await fs.mkdir(destDir, { recursive: true });

    // Fetch the CID (BAFY) that uniquely identifies files for this epoch
    console.log(`[getStandardIndexes] Fetching CID for epoch ${epochId}...`);
    const cidResponse = await fetch(`${BASE_URL}/${epochId}/epoch-${epochId}.cid`);
    if (!cidResponse.ok) {
      throw new Error(`Failed to fetch CID for epoch ${epochId}: ${cidResponse.status} ${cidResponse.statusText}`);
    }
    const cid = (await cidResponse.text()).trim();
    console.log(`[getStandardIndexes] CID for epoch ${epochId}: ${cid}`);

    // Build the full list of filenames for this epoch
    const filenames = INDEX_FILE_PATTERNS.map((pattern) =>
      pattern
        .replace(/__EPOCH__/g, epochId.toString())
        .replace(/__CID__/g, cid)
    );

    // Download each file sequentially to avoid overwhelming the remote server
    for (const filename of filenames) {
      const destPath = path.join(destDir, filename);

      // Skip existing files unless force is specified
      if (!force) {
        try {
          await fs.access(destPath);
          console.log(`[getStandardIndexes] File already exists, skipping: ${destPath}`);
          continue;
        } catch {
          // File does not exist – continue to download
        }
      }

      const fileUrl = `${BASE_URL}/${epochId}/${filename}`;
      console.log(`[getStandardIndexes] Downloading ${fileUrl} → ${destPath}`);

      const response = await fetch(fileUrl);
      if (!response.ok || !response.body) {
        throw new Error(`Failed to download ${fileUrl}: ${response.status} ${response.statusText}`);
      }

      // Stream the response directly to a file on disk
      await pipeline(response.body as unknown as NodeJS.ReadableStream, createWriteStream(destPath));
      console.log(`[getStandardIndexes] Download completed: ${destPath}`);
    }

    console.log(`[getStandardIndexes] All downloads finished for epoch ${epochId}`);
    await checkSource(epochId, source);
    await updateEpochStatus(epochId);
    console.log(`[getStandardIndexes] Completed for epoch ${epochId}`);
    return true;
  },
  schedule: async (args: GetStandardIndexesArgs): Promise<string> => {
    const job: Job = client.job(getStandardIndexesTask.name, args);
    job.queue = "local";
    job.reserveFor = 60 * 60 * 1; // 1 hour
    await job.push();
    return job.jid;
  },
};
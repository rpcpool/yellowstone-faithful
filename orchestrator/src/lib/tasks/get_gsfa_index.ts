// Refresh a single source

import { FileSystemSource } from "@/lib/data-sources/filesystem-source";
import { checkSource } from "@/lib/epochs";
import { getDataSource } from "@/lib/epochs/data-sources";
import { client } from "@/lib/infrastructure/faktory/faktory-client";
import { Task } from "@/lib/interfaces/task";

// import { createDecompressStream } from "@mongodb-js/zstd";
import { spawn } from "child_process";
import { promises as fs } from "fs";
import path from "path";
import { Readable } from "stream";
import { pipeline } from "stream/promises";
import tar, { Headers } from "tar-stream";
import { z } from "zod";
import { updateEpochStatus } from "../epochs/update-epoch-status";

export const getGsfaIndexArgsSchema = z.object({
  epochId: z.number(),
  force: z.boolean().optional().default(false),
});

type GetGsfaIndexArgs = z.infer<typeof getGsfaIndexArgsSchema>;

export const getGsfaIndexTask: Task<GetGsfaIndexArgs> = {
  name: "getGsfaIndex",
  description: "Fetches and extracts the GSFA index archive for a given epoch.",
  args: getGsfaIndexArgsSchema,
  requiredSource: "Local",
  requiredWorker: "local",
  validateArgs: (args: unknown): boolean => {
    return getGsfaIndexArgsSchema.safeParse(args).success;
  },
  run: async (args: GetGsfaIndexArgs): Promise<boolean> => {
    const { epochId, force } = args;
    const source = getDataSource("Local") as FileSystemSource;
    const checkResult = await checkSource(epochId, source);

    if (checkResult.gsfaFound && !force) {
      console.log(`[getGsfaIndex] GSFA already present for epoch ${epochId}. Skipping since force flag is false.`);
      return true;
    }

    const archiveUrl = await source.getEpochGsfaIndexArchiveUrl(epochId);
    console.log(`[getGsfaIndex] Fetching GSFA archive from: ${archiveUrl}`);

    const response = await fetch(archiveUrl);
    if (!response.ok) {
      throw new Error(`Failed to fetch GSFA archive: ${response.status} ${response.statusText}`);
    }

    const destDir = await source.getEpochFilePath(epochId);
    await fs.mkdir(destDir, { recursive: true });
    console.log(`[getGsfaIndex] Streaming extraction to ${destDir}`);

    const extract = tar.extract();

    // Spawn zstd CLI for on-the-fly decompression
    const zstd = spawn("zstd", ["-d", "-c"]);
    zstd.on("error", (err) => {
      console.error("[getGsfaIndex] Failed to spawn zstd:", err);
    });

    // Convert the fetch Response into a Node.js readable stream if necessary
    const responseStream =
      typeof (response.body as unknown as { pipe?: unknown })?.pipe === "function"
        ? (response.body as unknown as NodeJS.ReadableStream)
        : Readable.fromWeb(response.body as Parameters<typeof Readable.fromWeb>[0]);

    // Pipe the compressed data through zstd and then into tar-stream
    const pumpResponse = pipeline(responseStream, zstd.stdin);
    const pumpExtract = pipeline(zstd.stdout, extract);

    extract.on("entry", async (header: Headers, stream: NodeJS.ReadableStream, next: () => void) => {
      // Collapse duplicate first-level folder names (e.g. foo/foo/ → foo/)
      const segments = header.name.split("/");
      if (segments.length > 1 && segments[0] === segments[1]) {
        segments.shift();
      }

      const relativePath = segments.join("/");
      const filePath = path.join(destDir, relativePath);

      if (header.type === "directory") {
        await fs.mkdir(filePath, { recursive: true });
        stream.resume();
        return next();
      }

      await fs.mkdir(path.dirname(filePath), { recursive: true });

      const chunks: Buffer[] = [];
      stream.on("data", (chunk: Buffer) => chunks.push(chunk));
      stream.on("end", async () => {
        await fs.writeFile(filePath, Buffer.concat(chunks));
        next();
      });
    });

    await Promise.all([pumpResponse, pumpExtract]);

    console.log(`[getGsfaIndex] Extraction completed for epoch ${epochId}`);
    await checkSource(epochId, source);
    await updateEpochStatus(epochId);
    console.log(`[getGsfaIndex] Completed for epoch ${epochId}`);
    return true;
  },
  schedule: async (args: GetGsfaIndexArgs): Promise<string> => {
    const job = client.job(getGsfaIndexTask.name, args);
    job.queue = "local";
    job.reserveFor = 60 * 60 * 4; // 4 hours
    await job.push();
    return job.jid;
  },
};
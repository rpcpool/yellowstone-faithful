// build_indexes.ts
// Functions based on scripts/process.sh

import { PutObjectCommand, S3Client } from '@aws-sdk/client-s3';
import type { execa as execaType } from 'execa';
import fs from 'fs/promises';
import path from 'path';

/**
 * Configuration for index processing workers.
 */
export interface WorkerConfig {
  awsAccessKeyId: string;
  awsSecretAccessKey: string;
  awsEndpoint: string;
  epoch: string;
  dataDir?: string;
}

/**
 * Supported index types.
 */
export type IndexType = 'all' | 'gsfa';

/**
 * Returns the relevant paths for a given epoch and data directory.
 */
export function getPaths(epoch: string, dataDir = '/data') {
  const base = path.join(dataDir, `epoch-${epoch}`);
  return {
    base,
    car: path.join(base, `epoch-${epoch}.car`),
    indexes: path.join(base, 'indexes'),
  };
}

/**
 * Removes all epoch directories in dataDir except the current one.
 */
export async function cleanupDataDir(epoch: string, dataDir = '/data') {
  const entries = await fs.readdir(dataDir);
  await Promise.all(entries.map(async entry => {
    if (entry !== `epoch-${epoch}`) {
      await fs.rm(path.join(dataDir, entry), { recursive: true, force: true });
    }
  }));
}

/**
 * Downloads the CAR file for the given epoch to the specified path.
 */
export async function downloadCar(epoch: string, carPath: string) {
  const url = `https://files.old-faithful.net/${epoch}/epoch-${epoch}.car`;
  const outputDir = path.dirname(carPath);
  const outputFile = path.basename(carPath);
  
  await fs.mkdir(outputDir, { recursive: true });
  await fs.mkdir(path.join(outputDir, 'indexes'), { recursive: true });
  const { execa } = await import('execa');
  await (execa as typeof execaType)('aria2c', [
    '--auto-file-renaming=false',
    '--continue=true',
    '-x', '16',
    '-s', '16',
    '-d', outputDir,
    '-o', outputFile,
    url,
  ], { stdio: 'inherit' });
}

/**
 * Processes a specific index type for the given epoch.
 * Skips if the index already exists.
 */
export async function processIndex(
  type: IndexType,
  epoch: string,
  carPath: string,
  indexesDir: string
) {
  // if the car file doesn't exist, download it
  try {
    await fs.access(carPath);
  } catch {
    console.log(`Car file ${carPath} does not exist, downloading...`);
    await downloadCar(epoch, carPath);
  }

  await fs.mkdir(indexesDir, { recursive: true });
  const files = await fs.readdir(indexesDir);
  const { execa } = await import('execa');
  if (type === 'all') {
    if (files.some(f => f.endsWith('.index'))) return;
    await (execa as typeof execaType)(`${process.env.FAITHFUL_CLI_PATH}`, ['index', 'all', carPath, indexesDir], { stdio: 'inherit' });
  } else if (type === 'gsfa') {
    if (files.some(f => f.endsWith('.gsfa.indexdir'))) return;
    await (execa as typeof execaType)(`${process.env.FAITHFUL_CLI_PATH}`, ['index', 'gsfa', '--epoch', epoch, carPath, indexesDir], { stdio: 'inherit' });
  } else {
    throw new Error(`Unknown index type: ${type}`);
  }
}

/**
 * Verifies a specific index for the given epoch.
 */
export async function verifyIndex(
  type: IndexType,
  epoch: string,
  carPath: string,
  indexesDir: string
) {
  const { execa } = await import('execa');
  await (execa as typeof execaType)('faithful-cli', ['verify-index', type, carPath, indexesDir], { stdio: 'inherit' });
}

/**
 * Uploads all index files for the epoch to S3 using the AWS SDK.
 *
 * @param epoch - The epoch number as a string.
 * @param indexesDir - The directory containing index files.
 * @param awsEndpoint - The S3 endpoint URL (without protocol).
 * @param awsAccessKeyId - AWS access key ID.
 * @param awsSecretAccessKey - AWS secret access key.
 */
export async function uploadIndexes(
  epoch: string,
  indexesDir: string,
  awsEndpoint: string,
  awsAccessKeyId: string,
  awsSecretAccessKey: string
) {
  const files = await fs.readdir(indexesDir);
  const s3 = new S3Client({
    region: 'us-east-1',
    endpoint: `https://${awsEndpoint}`,
    credentials: {
      accessKeyId: awsAccessKeyId,
      secretAccessKey: awsSecretAccessKey,
    },
    forcePathStyle: true,
  });
  const bucket = 'solana-cars';
  await Promise.all(files.map(async (file) => {
    const localFile = path.join(indexesDir, file);
    const remoteKey = `epoch-${epoch}/indexes/${file}`;
    const body = await fs.readFile(localFile);
    await s3.send(new PutObjectCommand({
      Bucket: bucket,
      Key: remoteKey,
      Body: body,
    }));
  }));
}

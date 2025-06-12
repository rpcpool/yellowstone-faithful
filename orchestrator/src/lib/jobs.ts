import { JobStatus } from '@/generated/prisma/index.js';
import { prisma } from '@/lib/infrastructure/persistence/prisma';

export interface JobRecord {
  id: string;
  epochId: number;
  jobType: string;
  status: JobStatus;
  metadata?: Record<string, unknown>;
}

/**
 * Upserts a job record with the supplied data.
 */
export async function storeJobRecord(job: JobRecord): Promise<void> {
  await prisma.job.upsert({
    where: { id: job.id },
    update: {
      status: job.status,
      metadata: job.metadata,
      updatedAt: new Date(),
    },
    create: {
      id: job.id,
      epochId: job.epochId,
      jobType: job.jobType,
      status: job.status,
      metadata: job.metadata,
    },
  });
}

/**
 * Updates the status (and updatedAt) on an existing job record.
 */
export async function updateJobStatus(jobId: string, status: JobStatus): Promise<void> {
  await prisma.job.update({
    where: { id: jobId },
    data: {
      status,
      updatedAt: new Date(),
    },
  });
}

/**
 * Retrieves the most recent job matching the criteria.
 * Optionally filter by an array of statuses.
 */
export async function getLatestJob(
  epochId: number,
  jobType: string,
  statuses?: JobStatus[],
) {
  return prisma.job.findFirst({
    where: {
      epochId,
      jobType,
      ...(statuses && { status: { in: statuses } }),
    },
    orderBy: { createdAt: 'desc' },
  });
}

/**
 * Retrieves a list of recent jobs for an epoch/jobType (default last 5).
 */
export async function getRecentJobs(
  epochId: number,
  jobType: string,
  limit = 5,
) {
  return prisma.job.findMany({
    where: { epochId, jobType },
    orderBy: { createdAt: 'desc' },
    take: limit,
  });
} 
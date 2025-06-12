import { JobRepository } from '@/lib/domain/job/repositories/job-repository';
import { Job } from '@/lib/domain/job/entities/job';
import { JobStatus } from '@/lib/domain/job/value-objects/job-status';
import { EpochId } from '@/lib/domain/epoch/value-objects/epoch-id';
import { PrismaClient, Job as PrismaJob, JobStatus as PrismaJobStatus, Prisma } from '@/generated/prisma';
import { prisma } from '../persistence/prisma';

export class PrismaJobRepository implements JobRepository {
  private prisma: PrismaClient;

  constructor(prismaClient?: PrismaClient) {
    this.prisma = prismaClient || prisma;
  }

  async findById(id: string): Promise<Job | null> {
    const jobData = await this.prisma.job.findUnique({
      where: { id }
    });

    if (!jobData) {
      return null;
    }

    return this.toDomain(jobData);
  }

  async findByEpochId(epochId: EpochId): Promise<Job[]> {
    const jobs = await this.prisma.job.findMany({
      where: { epochId: epochId.getValue() },
      orderBy: { createdAt: 'desc' }
    });

    return jobs.map(job => this.toDomain(job));
  }

  async findByStatus(status: JobStatus): Promise<Job[]> {
    const jobs = await this.prisma.job.findMany({
      where: { status: this.toPrismaStatus(status) },
      orderBy: { createdAt: 'desc' }
    });

    return jobs.map(job => this.toDomain(job));
  }

  async findRecentByTypeAndStatus(
    epochId: EpochId,
    jobType: string,
    statuses: JobStatus[]
  ): Promise<Job | null> {
    const prismaStatuses = statuses.map(status => this.toPrismaStatus(status));

    const jobData = await this.prisma.job.findFirst({
      where: {
        epochId: epochId.getValue(),
        jobType,
        status: { in: prismaStatuses }
      },
      orderBy: { createdAt: 'desc' }
    });

    if (!jobData) {
      return null;
    }

    return this.toDomain(jobData);
  }

  async save(job: Job): Promise<void> {
    const data = {
      id: job.getId(),
      epochId: job.getEpochId().getValue(),
      jobType: job.getJobType(),
      status: this.toPrismaStatus(job.getStatus()),
      metadata: job.getMetadata() as Prisma.InputJsonValue,
      createdAt: job.getCreatedAt(),
      updatedAt: job.getUpdatedAt()
    };

    await this.prisma.job.upsert({
      where: { id: job.getId() },
      update: {
        status: data.status,
        metadata: data.metadata as Prisma.InputJsonValue,
        updatedAt: data.updatedAt
      },
      create: data
    });
  }

  async delete(id: string): Promise<void> {
    await this.prisma.job.delete({
      where: { id }
    });
  }

  private toDomain(jobData: PrismaJob): Job {
    return new Job({
      id: jobData.id,
      epochId: new EpochId(jobData.epochId),
      jobType: jobData.jobType,
      status: this.fromPrismaStatus(jobData.status),
      metadata: jobData.metadata as Record<string, unknown>,
      createdAt: jobData.createdAt,
      updatedAt: jobData.updatedAt
    });
  }

  private toPrismaStatus(status: JobStatus): PrismaJobStatus {
    const statusValue = status.getValue();
    switch (statusValue) {
      case 'queued':
        return PrismaJobStatus.queued;
      case 'processing':
        return PrismaJobStatus.processing;
      case 'completed':
        return PrismaJobStatus.completed;
      case 'failed':
        return PrismaJobStatus.failed;
      default:
        throw new Error(`Unknown job status: ${statusValue}`);
    }
  }

  private fromPrismaStatus(status: PrismaJobStatus): JobStatus {
    switch (status) {
      case PrismaJobStatus.queued:
        return JobStatus.Queued();
      case PrismaJobStatus.processing:
        return JobStatus.Processing();
      case PrismaJobStatus.completed:
        return JobStatus.Completed();
      case PrismaJobStatus.failed:
        return JobStatus.Failed();
      default:
        throw new Error(`Unknown Prisma job status: ${status}`);
    }
  }
}
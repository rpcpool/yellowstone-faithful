import { EpochId } from '@/lib/domain/epoch/value-objects/epoch-id';
import { Repository } from '@/lib/domain/shared/interfaces/repository';
import { Job } from '../entities/job';
import { JobStatus } from '../value-objects/job-status';

export interface JobRepository extends Repository<Job> {
  findByEpochId(epochId: EpochId): Promise<Job[]>;
  findByStatus(status: JobStatus): Promise<Job[]>;
  findRecentByTypeAndStatus(
    epochId: EpochId,
    jobType: string,
    statuses: JobStatus[]
  ): Promise<Job | null>;
  save(job: Job): Promise<Job>;
}
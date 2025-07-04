import { UseCase } from '@/lib/application/shared/interfaces/use-case';
import { EpochRepository } from '@/lib/domain/epoch/repositories/epoch-repository';
import { JobRepository } from '@/lib/domain/job/repositories/job-repository';
import { EpochId } from '@/lib/domain/epoch/value-objects/epoch-id';
import { Job } from '@/lib/domain/job/entities/job';
import { JobStatus } from '@/lib/domain/job/value-objects/job-status';
import { EpochDomainService } from '@/lib/domain/epoch/services/epoch-domain-service';

export interface RefreshEpochDto {
  epochId: number;
  triggeredBy?: string;
}

export interface RefreshEpochResponseDto {
  success: boolean;
  message: string;
  jobId?: string;
}

export class RefreshEpochUseCase implements UseCase<RefreshEpochDto, RefreshEpochResponseDto> {
  constructor(
    private readonly epochRepository: EpochRepository,
    private readonly jobRepository: JobRepository,
    private readonly epochDomainService: EpochDomainService,
    private readonly jobScheduler: { push: (jobType: string, payload: Record<string, unknown>) => Promise<void> }
  ) {}

  async execute(request: RefreshEpochDto): Promise<RefreshEpochResponseDto> {
    const { epochId, triggeredBy = 'manual' } = request;
    
    if (isNaN(epochId) || epochId < 0) {
      return {
        success: false,
        message: 'Invalid epoch ID'
      };
    }

    const epochIdVO = new EpochId(epochId);

    // Check if there's already a queued or processing refresh job for this epoch
    const existingJob = await this.jobRepository.findRecentByTypeAndStatus(
      epochIdVO,
      'RefreshEpoch',
      [JobStatus.Queued(), JobStatus.Processing()]
    );

    if (existingJob) {
      return {
        success: false,
        message: `Refresh job is already ${existingJob.getStatus().getValue()} for epoch ${epochId}`,
        jobId: existingJob.getId()
      };
    }

    try {
      // Generate unique job ID
      const jobId = `refresh-${epochId}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
      
      // Create job entity
      const job = new Job({
        id: jobId,
        epochId: epochIdVO,
        jobType: 'RefreshEpoch',
        status: JobStatus.Queued(),
        metadata: {
          epochId: epochId.toString(),
          triggeredBy,
          timestamp: new Date().toISOString()
        }
      });

      // Save job to database
      await this.jobRepository.save(job);

      // Schedule the job
      await this.jobScheduler.push('RefreshEpoch', {
        jobId,
        epochId: epochId.toString(),
        triggeredBy,
        timestamp: new Date().toISOString()
      });

      return {
        success: true,
        message: `Refresh job has been scheduled successfully for epoch ${epochId}. The process will scan all data sources and update the epoch status.`,
        jobId
      };
    } catch (error) {
      console.error('Failed to schedule refresh epoch job:', error);
      
      return {
        success: false,
        message: 'Failed to schedule refresh epoch job',
      };
    }
  }
}
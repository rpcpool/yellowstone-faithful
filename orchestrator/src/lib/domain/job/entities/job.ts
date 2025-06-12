import { Entity } from '@/lib/domain/shared/interfaces/entity';
import { EpochId } from '@/lib/domain/epoch/value-objects/epoch-id';
import { JobStatus } from '../value-objects/job-status';

export interface JobProps {
  id: string;
  epochId: EpochId;
  jobType: string;
  status: JobStatus;
  metadata?: Record<string, unknown>;
  createdAt?: Date;
  updatedAt?: Date;
}

/**
 * Entity representing a background job
 */
export class Job implements Entity<Job> {
  private readonly id: string;
  private readonly epochId: EpochId;
  private readonly jobType: string;
  private status: JobStatus;
  private metadata: Record<string, unknown>;
  private readonly createdAt: Date;
  private updatedAt: Date;

  constructor(props: JobProps) {
    this.id = props.id;
    this.epochId = props.epochId;
    this.jobType = props.jobType;
    this.status = props.status;
    this.metadata = props.metadata || {};
    this.createdAt = props.createdAt || new Date();
    this.updatedAt = props.updatedAt || new Date();
  }

  getId(): string {
    return this.id;
  }

  getEpochId(): EpochId {
    return this.epochId;
  }

  getJobType(): string {
    return this.jobType;
  }

  getStatus(): JobStatus {
    return this.status;
  }

  getMetadata(): Record<string, unknown> {
    return { ...this.metadata };
  }

  getCreatedAt(): Date {
    return this.createdAt;
  }

  getUpdatedAt(): Date {
    return this.updatedAt;
  }

  /**
   * Update job status
   */
  updateStatus(newStatus: JobStatus): void {
    if (!this.status.canTransitionTo(newStatus)) {
      throw new Error(
        `Cannot transition job from ${this.status.getValue()} to ${newStatus.getValue()}`
      );
    }
    
    this.status = newStatus;
    this.updatedAt = new Date();
  }

  /**
   * Update job metadata
   */
  updateMetadata(metadata: Record<string, unknown>): void {
    this.metadata = { ...this.metadata, ...metadata };
    this.updatedAt = new Date();
  }

  /**
   * Mark job as started
   */
  start(): void {
    this.updateStatus(JobStatus.Processing());
  }

  /**
   * Mark job as completed
   */
  complete(metadata?: Record<string, unknown>): void {
    if (metadata) {
      this.updateMetadata(metadata);
    }
    this.updateStatus(JobStatus.Completed());
  }

  /**
   * Mark job as failed
   */
  fail(error: string, metadata?: Record<string, unknown>): void {
    this.updateMetadata({
      ...metadata,
      error,
      failedAt: new Date().toISOString()
    });
    this.updateStatus(JobStatus.Failed());
  }

  /**
   * Check if job can be retried
   */
  canRetry(): boolean {
    return this.status.isFailed();
  }

  /**
   * Retry the job
   */
  retry(): void {
    if (!this.canRetry()) {
      throw new Error('Job cannot be retried in current state');
    }
    
    this.updateStatus(JobStatus.Queued());
    this.updateMetadata({
      retriedAt: new Date().toISOString(),
      retryCount: (this.metadata.retryCount || 0) + 1
    });
  }

  equals(other: Job): boolean {
    return this.id === other.id;
  }

  /**
   * Factory method to create a new job
   */
  static create(params: {
    id: string;
    epochId: EpochId;
    jobType: string;
    metadata?: Record<string, unknown>;
  }): Job {
    return new Job({
      id: params.id,
      epochId: params.epochId,
      jobType: params.jobType,
      status: JobStatus.Queued(),
      metadata: params.metadata
    });
  }
}
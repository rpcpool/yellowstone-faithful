import { ValueObject } from '@/lib/domain/shared/interfaces/value-object';

export enum JobStatusValue {
  Queued = 'queued',
  Processing = 'processing',
  Completed = 'completed',
  Failed = 'failed'
}

/**
 * Value object representing the status of a job
 */
export class JobStatus implements ValueObject<JobStatusValue> {
  private readonly value: JobStatusValue;

  private constructor(value: JobStatusValue) {
    this.value = value;
  }

  getValue(): JobStatusValue {
    return this.value;
  }

  equals(other: JobStatus): boolean {
    return this.value === other.value;
  }

  isQueued(): boolean {
    return this.value === JobStatusValue.Queued;
  }

  isProcessing(): boolean {
    return this.value === JobStatusValue.Processing;
  }

  isCompleted(): boolean {
    return this.value === JobStatusValue.Completed;
  }

  isFailed(): boolean {
    return this.value === JobStatusValue.Failed;
  }

  isTerminal(): boolean {
    return this.isCompleted() || this.isFailed();
  }

  canTransitionTo(newStatus: JobStatus): boolean {
    const transitions: Record<JobStatusValue, JobStatusValue[]> = {
      [JobStatusValue.Queued]: [JobStatusValue.Processing, JobStatusValue.Failed],
      [JobStatusValue.Processing]: [JobStatusValue.Completed, JobStatusValue.Failed],
      [JobStatusValue.Completed]: [], // Terminal state
      [JobStatusValue.Failed]: [JobStatusValue.Queued] // Can retry
    };

    return transitions[this.value]?.includes(newStatus.value) ?? false;
  }

  static Queued(): JobStatus {
    return new JobStatus(JobStatusValue.Queued);
  }

  static Processing(): JobStatus {
    return new JobStatus(JobStatusValue.Processing);
  }

  static Completed(): JobStatus {
    return new JobStatus(JobStatusValue.Completed);
  }

  static Failed(): JobStatus {
    return new JobStatus(JobStatusValue.Failed);
  }

  static fromString(value: string): JobStatus {
    if (!Object.values(JobStatusValue).includes(value as JobStatusValue)) {
      throw new Error(`Invalid job status: ${value}`);
    }
    return new JobStatus(value as JobStatusValue);
  }
}
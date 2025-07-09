import { ValueObject } from '@/lib/domain/shared/interfaces/value-object';

export enum EpochStatusValue {
  NotProcessed = 'NotProcessed',
  Processing = 'Processing',
  Indexed = 'Indexed',
  Complete = 'Complete'
}

/**
 * Value object representing the status of an epoch
 */
export class EpochStatus implements ValueObject<EpochStatusValue> {
  private readonly value: EpochStatusValue;

  private constructor(value: EpochStatusValue) {
    this.value = value;
  }

  getValue(): EpochStatusValue {
    return this.value;
  }

  equals(other: EpochStatus): boolean {
    return this.value === other.value;
  }

  isNotProcessed(): boolean {
    return this.value === EpochStatusValue.NotProcessed;
  }

  isProcessing(): boolean {
    return this.value === EpochStatusValue.Processing;
  }

  isIndexed(): boolean {
    return this.value === EpochStatusValue.Indexed;
  }

  isComplete(): boolean {
    return this.value === EpochStatusValue.Complete;
  }

  canTransitionTo(newStatus: EpochStatus): boolean {
    const transitions: Record<EpochStatusValue, EpochStatusValue[]> = {
      [EpochStatusValue.NotProcessed]: [EpochStatusValue.Processing],
      [EpochStatusValue.Processing]: [EpochStatusValue.Indexed, EpochStatusValue.NotProcessed],
      [EpochStatusValue.Indexed]: [EpochStatusValue.Complete, EpochStatusValue.Processing],
      [EpochStatusValue.Complete]: [EpochStatusValue.Processing] // Allow re-processing
    };

    return transitions[this.value]?.includes(newStatus.value) ?? false;
  }

  static NotProcessed(): EpochStatus {
    return new EpochStatus(EpochStatusValue.NotProcessed);
  }

  static Processing(): EpochStatus {
    return new EpochStatus(EpochStatusValue.Processing);
  }

  static Indexed(): EpochStatus {
    return new EpochStatus(EpochStatusValue.Indexed);
  }

  static Complete(): EpochStatus {
    return new EpochStatus(EpochStatusValue.Complete);
  }

  static fromString(value: string): EpochStatus {
    if (!Object.values(EpochStatusValue).includes(value as EpochStatusValue)) {
      throw new Error(`Invalid epoch status: ${value}`);
    }
    return new EpochStatus(value as EpochStatusValue);
  }
}
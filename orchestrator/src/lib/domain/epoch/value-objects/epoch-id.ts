import { ValueObject } from '@/lib/domain/shared/interfaces/value-object';

/**
 * Value object representing an Epoch identifier
 */
export class EpochId implements ValueObject<number> {
  private readonly value: number;

  constructor(value: number) {
    if (value < 0) {
      throw new Error('Epoch ID must be a non-negative number');
    }
    this.value = value;
  }

  getValue(): number {
    return this.value;
  }

  toString(): string {
    return this.value.toString();
  }

  equals(other: EpochId): boolean {
    return this.value === other.value;
  }

  static fromString(value: string): EpochId {
    const parsed = parseInt(value, 10);
    if (isNaN(parsed)) {
      throw new Error(`Invalid epoch ID: ${value}`);
    }
    return new EpochId(parsed);
  }
}
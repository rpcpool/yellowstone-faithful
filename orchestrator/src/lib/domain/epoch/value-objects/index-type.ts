import { ValueObject } from '@/lib/domain/shared/interfaces/value-object';

export enum IndexTypeValue {
  CidToOffsetAndSize = 'CidToOffsetAndSize',
  SigExists = 'SigExists',
  SigToCid = 'SigToCid',
  SlotToBlocktime = 'SlotToBlocktime',
  SlotToCid = 'SlotToCid'
}

/**
 * Value object representing the type of an epoch index
 */
export class IndexType implements ValueObject<IndexTypeValue> {
  private readonly value: IndexTypeValue;

  private constructor(value: IndexTypeValue) {
    this.value = value;
  }

  getValue(): IndexTypeValue {
    return this.value;
  }

  equals(other: IndexType): boolean {
    return this.value === other.value;
  }

  toKebabCase(): string {
    return this.value
      .replace(/([a-z])([A-Z])/g, '$1-$2')
      .toLowerCase();
  }

  static CidToOffsetAndSize(): IndexType {
    return new IndexType(IndexTypeValue.CidToOffsetAndSize);
  }

  static SigExists(): IndexType {
    return new IndexType(IndexTypeValue.SigExists);
  }

  static SigToCid(): IndexType {
    return new IndexType(IndexTypeValue.SigToCid);
  }

  static SlotToBlocktime(): IndexType {
    return new IndexType(IndexTypeValue.SlotToBlocktime);
  }

  static SlotToCid(): IndexType {
    return new IndexType(IndexTypeValue.SlotToCid);
  }

  static fromString(value: string): IndexType {
    if (!Object.values(IndexTypeValue).includes(value as IndexTypeValue)) {
      throw new Error(`Invalid index type: ${value}`);
    }
    return new IndexType(value as IndexTypeValue);
  }

  static all(): IndexType[] {
    return Object.values(IndexTypeValue).map(value => new IndexType(value));
  }
}
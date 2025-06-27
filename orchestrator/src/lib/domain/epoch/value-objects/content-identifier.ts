import { ValueObject } from '@/lib/domain/shared/interfaces/value-object';

/**
 * Value object representing a Content Identifier (CID)
 */
export class ContentIdentifier implements ValueObject<string> {
  private readonly value: string;

  constructor(value: string) {
    if (!value || value.trim().length === 0) {
      throw new Error('Content identifier cannot be empty');
    }
    this.value = value.trim();
  }

  getValue(): string {
    return this.value;
  }

  equals(other: ContentIdentifier): boolean {
    return this.value === other.value;
  }

  toString(): string {
    return this.value;
  }
}
import { ValueObject } from '@/lib/domain/shared/interfaces/value-object';

export enum DataSourceTypeValue {
  S3 = 's3',
  HTTP = 'http',
  FILESYSTEM = 'filesystem'
}

/**
 * Value object representing the type of a data source
 */
export class DataSourceType implements ValueObject<DataSourceTypeValue> {
  private readonly value: DataSourceTypeValue;

  private constructor(value: DataSourceTypeValue) {
    this.value = value;
  }

  getValue(): DataSourceTypeValue {
    return this.value;
  }

  equals(other: DataSourceType): boolean {
    return this.value === other.value;
  }

  toString(): string {
    return this.value;
  }

  static S3(): DataSourceType {
    return new DataSourceType(DataSourceTypeValue.S3);
  }

  static HTTP(): DataSourceType {
    return new DataSourceType(DataSourceTypeValue.HTTP);
  }

  static FILESYSTEM(): DataSourceType {
    return new DataSourceType(DataSourceTypeValue.FILESYSTEM);
  }

  static fromString(value: string): DataSourceType {
    if (!Object.values(DataSourceTypeValue).includes(value as DataSourceTypeValue)) {
      throw new Error(`Invalid data source type: ${value}`);
    }
    return new DataSourceType(value as DataSourceTypeValue);
  }
}
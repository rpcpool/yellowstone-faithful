import { Entity } from '@/lib/domain/shared/interfaces/entity';
import { EpochId } from '../value-objects/epoch-id';
import { IndexType } from '../value-objects/index-type';

export interface EpochIndexProps {
  id?: string;
  epochId: EpochId;
  type: IndexType;
  size: bigint;
  status: string;
  location: string;
  source: string;
  createdAt?: Date;
  updatedAt?: Date;
}

/**
 * Entity representing an index for an epoch
 */
export class EpochIndex implements Entity<EpochIndex> {
  private readonly id?: string;
  private readonly epochId: EpochId;
  private readonly type: IndexType;
  private readonly size: bigint;
  private readonly status: string;
  private readonly location: string;
  private readonly source: string;
  private readonly createdAt: Date;
  private readonly updatedAt: Date;

  constructor(props: EpochIndexProps) {
    this.id = props.id;
    this.epochId = props.epochId;
    this.type = props.type;
    this.size = props.size;
    this.status = props.status;
    this.location = props.location;
    this.source = props.source;
    this.createdAt = props.createdAt || new Date();
    this.updatedAt = props.updatedAt || new Date();
  }

  getId(): string | undefined {
    return this.id;
  }

  getEpochId(): EpochId {
    return this.epochId;
  }

  getType(): IndexType {
    return this.type;
  }

  getSize(): bigint {
    return this.size;
  }

  getSizeInMB(): number {
    return Number(this.size) / (1024 * 1024);
  }

  getStatus(): string {
    return this.status;
  }

  getLocation(): string {
    return this.location;
  }

  getSource(): string {
    return this.source;
  }

  getCreatedAt(): Date {
    return this.createdAt;
  }

  getUpdatedAt(): Date {
    return this.updatedAt;
  }

  equals(other: EpochIndex): boolean {
    return (
      this.epochId.equals(other.epochId) &&
      this.type.equals(other.type) &&
      this.source === other.source
    );
  }

  /**
   * Creates a unique key for this index
   */
  getUniqueKey(): string {
    return `${this.epochId.toString()}-${this.type.getValue()}-${this.source}`;
  }
}
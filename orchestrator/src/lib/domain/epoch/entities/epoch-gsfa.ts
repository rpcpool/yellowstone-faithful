import { Entity } from '@/lib/domain/shared/interfaces/entity';
import { EpochId } from '../value-objects/epoch-id';

export interface EpochGsfaProps {
  id?: number;
  epochId: EpochId;
  exists: boolean;
  location: string;
  createdAt?: Date;
  updatedAt?: Date;
}

/**
 * Entity representing a GSFA (GetSignaturesForAddress) index for an epoch
 */
export class EpochGsfa implements Entity<EpochGsfa> {
  private readonly id?: number;
  private readonly epochId: EpochId;
  private readonly _exists: boolean;
  private readonly location: string;
  private readonly createdAt: Date;
  private readonly updatedAt: Date;

  constructor(props: EpochGsfaProps) {
    this.id = props.id;
    this.epochId = props.epochId;
    this._exists = props.exists;
    this.location = props.location;
    this.createdAt = props.createdAt || new Date();
    this.updatedAt = props.updatedAt || new Date();
  }

  getId(): number | undefined {
    return this.id;
  }

  getEpochId(): EpochId {
    return this.epochId;
  }

  exists(): boolean {
    return this._exists;
  }

  getLocation(): string {
    return this.location;
  }

  getCreatedAt(): Date {
    return this.createdAt;
  }

  getUpdatedAt(): Date {
    return this.updatedAt;
  }

  equals(other: EpochGsfa): boolean {
    return this.epochId.equals(other.epochId) && this.location === other.location;
  }
}
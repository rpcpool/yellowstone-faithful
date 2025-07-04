import { AggregateRoot, DomainEvent } from '@/lib/domain/shared/interfaces/entity';
import { EpochId } from '../value-objects/epoch-id';
import { EpochStatus } from '../value-objects/epoch-status';
import { ContentIdentifier } from '../value-objects/content-identifier';
import { IndexType } from '../value-objects/index-type';
import { EpochIndex } from './epoch-index';
import { EpochGsfa } from './epoch-gsfa';

export interface EpochProps {
  id: EpochId;
  status: EpochStatus;
  cid?: ContentIdentifier;
  indexes: EpochIndex[];
  gsfaIndexes: EpochGsfa[];
  createdAt?: Date;
  updatedAt?: Date;
}

/**
 * Epoch aggregate root - represents a blockchain epoch with its indexes
 */
export class Epoch implements AggregateRoot<Epoch> {
  private readonly id: EpochId;
  private status: EpochStatus;
  private cid?: ContentIdentifier;
  private indexes: Map<string, EpochIndex>;
  private gsfaIndexes: EpochGsfa[];
  private readonly createdAt: Date;
  private updatedAt: Date;
  private uncommittedEvents: DomainEvent[] = [];

  constructor(props: EpochProps) {
    this.id = props.id;
    this.status = props.status;
    this.cid = props.cid;
    this.indexes = new Map(props.indexes.map(idx => [idx.getUniqueKey(), idx]));
    this.gsfaIndexes = props.gsfaIndexes;
    this.createdAt = props.createdAt || new Date();
    this.updatedAt = props.updatedAt || new Date();
  }

  getId(): EpochId {
    return this.id;
  }

  getStatus(): EpochStatus {
    return this.status;
  }

  getCid(): ContentIdentifier | undefined {
    return this.cid;
  }

  getIndexes(): EpochIndex[] {
    return Array.from(this.indexes.values());
  }

  getGsfaIndexes(): EpochGsfa[] {
    return this.gsfaIndexes;
  }

  getCreatedAt(): Date {
    return this.createdAt;
  }

  getUpdatedAt(): Date {
    return this.updatedAt;
  }

  /**
   * Add or update an index for this epoch
   */
  addIndex(index: EpochIndex): void {
    if (!index.getEpochId().equals(this.id)) {
      throw new Error('Index does not belong to this epoch');
    }
    
    this.indexes.set(index.getUniqueKey(), index);
    this.updatedAt = new Date();
    
    this.addEvent({
      aggregateId: this.id.toString(),
      eventType: 'EpochIndexAdded',
      occurredAt: new Date(),
      payload: {
        epochId: this.id.getValue(),
        indexType: index.getType().getValue(),
        sourceId: index.getSourceId(),
        size: index.getSize().toString()
      }
    });
  }

  /**
   * Add or update a GSFA index
   */
  addGsfaIndex(gsfa: EpochGsfa): void {
    if (!gsfa.getEpochId().equals(this.id)) {
      throw new Error('GSFA index does not belong to this epoch');
    }
    
    // Remove any existing GSFA index with the same location
    this.gsfaIndexes = this.gsfaIndexes.filter(g => g.getLocation() !== gsfa.getLocation());
    this.gsfaIndexes.push(gsfa);
    this.updatedAt = new Date();
    
    this.addEvent({
      aggregateId: this.id.toString(),
      eventType: 'EpochGsfaIndexAdded',
      occurredAt: new Date(),
      payload: {
        epochId: this.id.getValue(),
        location: gsfa.getLocation(),
        exists: gsfa.exists()
      }
    });
  }

  /**
   * Set the CID for this epoch
   */
  setCid(cid: ContentIdentifier): void {
    this.cid = cid;
    this.updatedAt = new Date();
    
    this.addEvent({
      aggregateId: this.id.toString(),
      eventType: 'EpochCidSet',
      occurredAt: new Date(),
      payload: {
        epochId: this.id.getValue(),
        cid: cid.getValue()
      }
    });
  }

  /**
   * Transition to a new status
   */
  transitionToStatus(newStatus: EpochStatus): void {
    if (!this.status.canTransitionTo(newStatus)) {
      throw new Error(
        `Cannot transition from ${this.status.getValue()} to ${newStatus.getValue()}`
      );
    }
    
    const oldStatus = this.status;
    this.status = newStatus;
    this.updatedAt = new Date();
    
    this.addEvent({
      aggregateId: this.id.toString(),
      eventType: 'EpochStatusChanged',
      occurredAt: new Date(),
      payload: {
        epochId: this.id.getValue(),
        oldStatus: oldStatus.getValue(),
        newStatus: newStatus.getValue()
      }
    });
  }

  /**
   * Calculate and update the epoch status based on available indexes
   */
  updateStatusBasedOnIndexes(): void {
    const allIndexTypes = IndexType.all();
    const indexTypeCounts = new Map<string, number>();
    
    // Count indexes by type
    for (const index of this.indexes.values()) {
      const type = index.getType().getValue();
      indexTypeCounts.set(type, (indexTypeCounts.get(type) || 0) + 1);
    }
    
    // Check if we have all regular index types
    const hasAllRegularIndexes = allIndexTypes.every(
      type => (indexTypeCounts.get(type.getValue()) || 0) > 0
    );
    
    // Check if we have any GSFA index
    const hasGsfaIndex = this.gsfaIndexes.some(gsfa => gsfa.exists());
    
    // Check if we have any indexes at all
    const hasSomeIndexes = indexTypeCounts.size > 0;
    
    let newStatus: EpochStatus;
    
    if (!hasSomeIndexes) {
      newStatus = EpochStatus.NotProcessed();
    } else if (hasAllRegularIndexes && hasGsfaIndex) {
      newStatus = EpochStatus.Complete();
    } else if (hasAllRegularIndexes) {
      newStatus = EpochStatus.Indexed();
    } else {
      newStatus = EpochStatus.Processing();
    }
    
    if (!this.status.equals(newStatus) && this.status.canTransitionTo(newStatus)) {
      this.transitionToStatus(newStatus);
    }
  }

  /**
   * Check if epoch has a specific index type from a source
   */
  hasIndex(type: IndexType, source: string): boolean {
    const key = `${this.id.toString()}-${type.getValue()}-${source}`;
    return this.indexes.has(key);
  }

  /**
   * Check if epoch has all standard indexes from at least one source
   */
  hasAllStandardIndexes(): boolean {
    const allTypes = IndexType.all();
    return allTypes.every(type => {
      // Check if at least one source has this index type
      return Array.from(this.indexes.values()).some(
        index => index.getType().equals(type)
      );
    });
  }

  /**
   * Check if epoch has GSFA index
   */
  hasGsfaIndex(): boolean {
    return this.gsfaIndexes.some(gsfa => gsfa.exists());
  }

  equals(other: Epoch): boolean {
    return this.id.equals(other.id);
  }

  getUncommittedEvents(): DomainEvent[] {
    return [...this.uncommittedEvents];
  }

  markEventsAsCommitted(): void {
    this.uncommittedEvents = [];
  }

  private addEvent(event: DomainEvent): void {
    this.uncommittedEvents.push(event);
  }

  /**
   * Factory method to create a new epoch
   */
  static create(id: EpochId): Epoch {
    const epoch = new Epoch({
      id,
      status: EpochStatus.NotProcessed(),
      indexes: [],
      gsfaIndexes: []
    });
    
    epoch.addEvent({
      aggregateId: id.toString(),
      eventType: 'EpochCreated',
      occurredAt: new Date(),
      payload: {
        epochId: id.getValue()
      }
    });
    
    return epoch;
  }
}
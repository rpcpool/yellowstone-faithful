/**
 * Base interface for all domain entities
 */
export interface Entity<T> {
  equals(other: Entity<T>): boolean;
}

/**
 * Base interface for aggregate roots
 */
export interface AggregateRoot<T> extends Entity<T> {
  getUncommittedEvents(): DomainEvent[];
  markEventsAsCommitted(): void;
}

/**
 * Base interface for domain events
 */
export interface DomainEvent {
  aggregateId: string;
  eventType: string;
  occurredAt: Date;
  payload: unknown;
}
import { DomainEvent } from '@/lib/domain/shared/interfaces/entity';

/**
 * Interface for publishing domain events
 */
export interface EventPublisher {
  publish(events: DomainEvent[]): Promise<void>;
}
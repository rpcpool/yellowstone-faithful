import { DomainEvent } from '@/lib/domain/shared/interfaces/entity';

export interface EpochCreatedEvent extends DomainEvent {
  eventType: 'EpochCreated';
  payload: {
    epochId: number;
  };
}

export interface EpochStatusChangedEvent extends DomainEvent {
  eventType: 'EpochStatusChanged';
  payload: {
    epochId: number;
    oldStatus: string;
    newStatus: string;
  };
}

export interface EpochCidSetEvent extends DomainEvent {
  eventType: 'EpochCidSet';
  payload: {
    epochId: number;
    cid: string;
  };
}

export interface EpochIndexAddedEvent extends DomainEvent {
  eventType: 'EpochIndexAdded';
  payload: {
    epochId: number;
    indexType: string;
    source: string;
    size: string;
  };
}

export interface EpochGsfaIndexAddedEvent extends DomainEvent {
  eventType: 'EpochGsfaIndexAdded';
  payload: {
    epochId: number;
    location: string;
    exists: boolean;
  };
}

export type EpochDomainEvent = 
  | EpochCreatedEvent 
  | EpochStatusChangedEvent 
  | EpochCidSetEvent 
  | EpochIndexAddedEvent 
  | EpochGsfaIndexAddedEvent;
import { Repository } from '@/lib/domain/shared/interfaces/repository';
import { Epoch } from '../entities/epoch';
import { EpochId } from '../value-objects/epoch-id';
import { EpochStatus } from '../value-objects/epoch-status';

export interface EpochRepository extends Repository<Epoch> {
  /**
   * Find an epoch by its ID
   */
  findById(id: string): Promise<Epoch | null>;
  
  /**
   * Find an epoch by its epoch ID value object
   */
  findByEpochId(epochId: EpochId): Promise<Epoch | null>;
  
  /**
   * Find epochs by status
   */
  findByStatus(status: EpochStatus): Promise<Epoch[]>;
  
  /**
   * Find epochs within a range
   */
  findByRange(start: EpochId, end: EpochId): Promise<Epoch[]>;
  
  /**
   * Find epochs with pagination
   */
  findWithPagination(params: {
    page: number;
    pageSize: number;
    search?: string;
    status?: EpochStatus;
  }): Promise<{
    epochs: Epoch[];
    totalCount: number;
  }>;
  
  /**
   * Get the latest epoch
   */
  findLatest(): Promise<Epoch | null>;
  
  /**
   * Save an epoch (create or update)
   */
  save(epoch: Epoch): Promise<void>;
  
  /**
   * Delete an epoch
   */
  delete(id: string): Promise<void>;
  
  /**
   * Check if an epoch exists
   */
  exists(epochId: EpochId): Promise<boolean>;
  
  /**
   * Find all epochs
   */
  findAll(): Promise<Epoch[]>;
}
import { EpochId } from '@/lib/domain/epoch/value-objects/epoch-id';
import { IndexType } from '@/lib/domain/epoch/value-objects/index-type';
import { DataSourceType } from '../value-objects/data-source-type';

/**
 * Domain interface for data sources that provide epoch data
 */
export interface DataSource {
  getName(): string;
  getType(): DataSourceType;
  
  /**
   * Check if an epoch exists in this data source
   */
  epochExists(epochId: EpochId): Promise<boolean>;
  
  /**
   * Check if a specific index exists for an epoch
   */
  epochIndexExists(epochId: EpochId, indexType: IndexType): Promise<boolean>;
  
  /**
   * Check if GSFA index exists for an epoch
   */
  epochGsfaIndexExists(epochId: EpochId): Promise<boolean>;
  
  /**
   * Get statistics for an epoch index
   */
  getEpochIndexStats(epochId: EpochId, indexType: IndexType): Promise<{
    size: number;
    location: string;
  }>;
  
  /**
   * Get the CID for an epoch
   */
  getEpochCid(epochId: EpochId): Promise<string>;
  
  /**
   * Get URLs for various epoch resources
   */
  getEpochCarUrl(epochId: EpochId): Promise<string>;
  getEpochIndexUrl(epochId: EpochId, indexType: IndexType): Promise<string>;
  getEpochGsfaUrl(epochId: EpochId): Promise<string>;
  getEpochGsfaIndexArchiveUrl(epochId: EpochId): Promise<string>;
}
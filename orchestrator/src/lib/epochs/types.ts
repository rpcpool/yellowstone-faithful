import { EpochStatus, IndexStatus, IndexType } from '../../generated/prisma/index.js';

export { EpochStatus, IndexStatus, IndexType };

export interface EpochExistsResult {
  source: string;
  exists: boolean;
}

export interface IndexExistsResult {
  source: string;
  exists: boolean;
}

export interface IndexResult {
  type: IndexType;
  size: number;
  availableIn: string[];
}

export interface DataSourceResult<T> {
  source: string;
  result: T | null;
}

export interface SourceCheckResult {
  source: string;
  indexesFound: number;
  gsfaFound: boolean;
  success: boolean;
} 
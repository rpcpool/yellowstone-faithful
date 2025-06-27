import { Repository } from '@/lib/domain/shared/interfaces/repository';
import { Source } from '../entities/source';
import { DataSourceType } from '@/generated/prisma';

export interface SourceFilters {
  type?: DataSourceType;
  enabled?: boolean;
  search?: string;
}

export interface PaginationOptions {
  page: number;
  pageSize: number;
}

export interface PaginatedResult<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

export interface SourceRepository extends Repository<Source> {
  findById(id: string): Promise<Source | null>;
  findByName(name: string): Promise<Source | null>;
  findAll(filters?: SourceFilters, pagination?: PaginationOptions): Promise<PaginatedResult<Source>>;
  save(source: Source): Promise<void>;
  delete(id: string): Promise<void>;
  exists(id: string): Promise<boolean>;
  existsByName(name: string): Promise<boolean>;
  countByType(type: DataSourceType): Promise<number>;
  findEnabled(): Promise<Source[]>;
}
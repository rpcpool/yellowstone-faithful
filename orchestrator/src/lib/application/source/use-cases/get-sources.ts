import { SourceRepository, SourceFilters, PaginationOptions } from '@/lib/domain/source/repositories/source-repository';
import { DataSourceType } from '@/generated/prisma';

export interface GetSourcesRequest {
  page?: number;
  pageSize?: number;
  type?: DataSourceType;
  enabled?: boolean;
  search?: string;
}

export interface SourceDto {
  id: string;
  name: string;
  type: DataSourceType;
  configuration: Record<string, unknown>;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface GetSourcesResponse {
  sources: SourceDto[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

export class GetSourcesUseCase {
  constructor(private readonly sourceRepository: SourceRepository) {}

  async execute(request: GetSourcesRequest): Promise<GetSourcesResponse> {
    const filters: SourceFilters = {
      type: request.type,
      enabled: request.enabled,
      search: request.search
    };

    const pagination: PaginationOptions = {
      page: request.page || 1,
      pageSize: request.pageSize || 10
    };

    const result = await this.sourceRepository.findAll(filters, pagination);

    return {
      sources: result.items.map(source => ({
        id: source.id,
        name: source.name,
        type: source.type,
        configuration: source.configuration,
        enabled: source.enabled,
        createdAt: source.createdAt.toISOString(),
        updatedAt: source.updatedAt.toISOString()
      })),
      total: result.total,
      page: result.page,
      pageSize: result.pageSize,
      totalPages: result.totalPages
    };
  }
}
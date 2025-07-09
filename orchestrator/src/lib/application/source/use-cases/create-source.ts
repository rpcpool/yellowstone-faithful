import { SourceDomainService } from '@/lib/domain/source/services/source-domain-service';
import { SourceConfiguration } from '@/lib/domain/source/entities/source';
import { DataSourceType } from '@/generated/prisma';
import { SourceDto } from './get-sources';

export interface CreateSourceRequest {
  name: string;
  type: DataSourceType;
  configuration: SourceConfiguration;
  enabled?: boolean;
}

export interface CreateSourceResponse {
  source: SourceDto;
}

export class CreateSourceUseCase {
  constructor(private readonly sourceDomainService: SourceDomainService) {}

  async execute(request: CreateSourceRequest): Promise<CreateSourceResponse> {
    const source = await this.sourceDomainService.createSource(
      request.name,
      request.type,
      request.configuration,
      request.enabled ?? true
    );

    return {
      source: {
        id: source.id,
        name: source.name,
        type: source.type,
        configuration: source.configuration,
        enabled: source.enabled,
        createdAt: source.createdAt.toISOString(),
        updatedAt: source.updatedAt.toISOString()
      }
    };
  }
}
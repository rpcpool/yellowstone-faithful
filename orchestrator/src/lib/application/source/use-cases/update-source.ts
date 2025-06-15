import { SourceDomainService } from '@/lib/domain/source/services/source-domain-service';
import { SourceConfiguration } from '@/lib/domain/source/entities/source';
import { SourceDto } from './get-sources';

export interface UpdateSourceRequest {
  id: string;
  name?: string;
  configuration?: SourceConfiguration;
  enabled?: boolean;
}

export interface UpdateSourceResponse {
  source: SourceDto;
}

export class UpdateSourceUseCase {
  constructor(private readonly sourceDomainService: SourceDomainService) {}

  async execute(request: UpdateSourceRequest): Promise<UpdateSourceResponse> {
    const source = await this.sourceDomainService.updateSource(request.id, {
      name: request.name,
      configuration: request.configuration,
      enabled: request.enabled
    });

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
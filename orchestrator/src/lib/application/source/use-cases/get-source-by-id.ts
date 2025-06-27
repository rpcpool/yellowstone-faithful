import { SourceRepository } from '@/lib/domain/source/repositories/source-repository';
import { SourceDto } from './get-sources';

export interface GetSourceByIdRequest {
  id: string;
}

export interface GetSourceByIdResponse {
  source: SourceDto | null;
}

export class GetSourceByIdUseCase {
  constructor(private readonly sourceRepository: SourceRepository) {}

  async execute(request: GetSourceByIdRequest): Promise<GetSourceByIdResponse> {
    const source = await this.sourceRepository.findById(request.id);

    if (!source) {
      return { source: null };
    }

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
import { SourceDomainService } from '@/lib/domain/source/services/source-domain-service';

export interface DeleteSourceRequest {
  id: string;
}

export interface DeleteSourceResponse {
  success: boolean;
}

export class DeleteSourceUseCase {
  constructor(private readonly sourceDomainService: SourceDomainService) {}

  async execute(request: DeleteSourceRequest): Promise<DeleteSourceResponse> {
    await this.sourceDomainService.deleteSource(request.id);
    return { success: true };
  }
}
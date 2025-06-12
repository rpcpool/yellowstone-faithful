import { UseCase } from '@/lib/application/shared/interfaces/use-case';
import { EpochRepository } from '@/lib/domain/epoch/repositories/epoch-repository';
import { EpochStatus } from '@/lib/domain/epoch/value-objects/epoch-status';
import { GetEpochsDto, GetEpochsResponseDto, EpochDto } from '../dto/epoch-dto';
import { EpochMapper } from '../mappers/epoch-mapper';

export class GetEpochsUseCase implements UseCase<GetEpochsDto, GetEpochsResponseDto> {
  constructor(private readonly epochRepository: EpochRepository) {}

  async execute(request: GetEpochsDto): Promise<GetEpochsResponseDto> {
    const page = request.page || 1;
    const pageSize = request.pageSize || 20;
    
    // Validate pagination parameters
    if (page < 1 || pageSize < 1 || pageSize > 100) {
      throw new Error('Invalid pagination parameters');
    }
    
    // Parse status if provided
    let status: EpochStatus | undefined;
    if (request.status && request.status !== 'all') {
      status = EpochStatus.fromString(request.status);
    }
    
    // Get epochs from repository
    const result = await this.epochRepository.findWithPagination({
      page,
      pageSize,
      search: request.search,
      status
    });
    
    // Map domain entities to DTOs
    const epochDtos: EpochDto[] = result.epochs.map(epoch => 
      EpochMapper.toDto(epoch)
    );
    
    return {
      epochs: epochDtos,
      pagination: {
        page,
        pageSize,
        totalCount: result.totalCount,
        totalPages: Math.ceil(result.totalCount / pageSize)
      }
    };
  }
}
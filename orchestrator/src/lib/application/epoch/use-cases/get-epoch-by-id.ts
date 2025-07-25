import { UseCase } from '@/lib/application/shared/interfaces/use-case';
import { EpochRepository } from '@/lib/domain/epoch/repositories/epoch-repository';
import { EpochId } from '@/lib/domain/epoch/value-objects/epoch-id';
import { EpochDto } from '../dto/epoch-dto';
import { EpochMapper } from '../mappers/epoch-mapper';

export interface GetEpochByIdDto {
  epochId: number;
}

export interface GetEpochByIdResponseDto {
  epoch: EpochDto | null;
}

export class GetEpochByIdUseCase implements UseCase<GetEpochByIdDto, GetEpochByIdResponseDto> {
  constructor(private readonly epochRepository: EpochRepository) {}

  async execute(request: GetEpochByIdDto): Promise<GetEpochByIdResponseDto> {
    const epochId = new EpochId(request.epochId);
    const epoch = await this.epochRepository.findByEpochId(epochId);
    
    if (!epoch) {
      return { epoch: null };
    }
    
    return {
      epoch: EpochMapper.toDto(epoch)
    };
  }
}
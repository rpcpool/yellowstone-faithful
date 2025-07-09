import { UseCase } from '@/lib/application/shared/interfaces/use-case';
import { EpochRepository } from '@/lib/domain/epoch/repositories/epoch-repository';
import { EpochDto } from '../dto/epoch-dto';
import { EpochMapper } from '../mappers/epoch-mapper';

export interface GetEpochsRangeDto {
  start: number;
  end: number;
}

export interface GetEpochsRangeResponseDto {
  epochs: EpochDto[];
}

export class GetEpochsRangeUseCase implements UseCase<GetEpochsRangeDto, GetEpochsRangeResponseDto> {
  constructor(private readonly epochRepository: EpochRepository) {}

  // TODO: Needs to implement actual ranging instead of just fetching all epochs and filtering
  async execute(request: GetEpochsRangeDto): Promise<GetEpochsRangeResponseDto> {
    const { start, end } = request;
    
    if (start < 0 || end < start) {
      throw new Error('Invalid range parameters');
    }
    
    const allEpochs = await this.epochRepository.findAll();
    
    const epochsInRange = allEpochs
      .filter(epoch => {
        const id = epoch.getId().getValue();
        return id >= start && id <= end;
      })
      .sort((a, b) => a.getId().getValue() - b.getId().getValue());
    
    const epochDtos = epochsInRange.map(epoch => EpochMapper.toDto(epoch));
    
    return { epochs: epochDtos };
  }
}
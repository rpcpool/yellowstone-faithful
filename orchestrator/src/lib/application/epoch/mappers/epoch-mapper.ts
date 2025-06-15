import { Epoch } from '@/lib/domain/epoch/entities/epoch';
import { EpochDto } from '../dto/epoch-dto';

export class EpochMapper {
  static toDto(epoch: Epoch): EpochDto {
    const indexes = epoch.getIndexes();
    const gsfaIndexes = epoch.getGsfaIndexes();

    return {
      id: epoch.getId().getValue(),
      epoch: epoch.getId().toString(),
      status: epoch.getStatus().getValue(),
      cid: epoch.getCid()?.getValue(),
      createdAt: epoch.getCreatedAt(),
      updatedAt: epoch.getUpdatedAt(),
      indexes: indexes.map(index => ({
        epoch: index.getEpochId().toString(),
        type: index.getType().getValue(),
        size: index.getSize().toString(),
        status: index.getStatus(),
        location: index.getLocation(),
        source: index.getSourceId(), // TODO: Map to source name
        createdAt: index.getCreatedAt(),
        updatedAt: index.getUpdatedAt()
      })),
      gsfaIndexes: gsfaIndexes.map(gsfa => ({
        id: gsfa.getId(),
        epoch: gsfa.getEpochId().toString(),
        exists: gsfa.exists(),
        location: gsfa.getLocation(),
        createdAt: gsfa.getCreatedAt(),
        updatedAt: gsfa.getUpdatedAt()
      }))
    };
  }

  static toDtoWithIndexObjects(epoch: Epoch): {
    hasData: boolean;
    objects: Array<{
      name: string;
      size: number;
      status: string;
      location?: string;
    }>;
    epochStatus: string;
  } {
    const indexes = epoch.getIndexes();
    
    const objects = indexes.length > 0 
      ? indexes.map(index => ({
          name: `${index.getType().getValue()}-${epoch.getId().getValue()}`,
          size: Number(index.getSize()),
          status: index.getStatus(),
          location: index.getLocation()
        }))
      : [{ 
          name: `epoch-${epoch.getId().getValue()}`, 
          size: 0, 
          status: epoch.getStatus().getValue() 
        }];
    
    return {
      hasData: true,
      objects,
      epochStatus: epoch.getStatus().getValue()
    };
  }
}
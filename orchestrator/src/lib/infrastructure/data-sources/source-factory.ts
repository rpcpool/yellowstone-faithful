import { DataSource } from '@/lib/interfaces/data-source';
import { 
  Source, 
  S3Configuration, 
  HTTPConfiguration, 
  FilesystemConfiguration
} from '@/lib/domain/source/entities/source';
import { DataSourceType } from '@/generated/prisma';
import { createS3Source } from './s3-source';
import { createHTTPSource } from './http-source';
import { createFilesystemSource } from './filesystem-source';

export class SourceFactory {
  static createDataSource(source: Source): DataSource {
    const config = source.getTypedConfiguration();

    switch (source.type) {
      case DataSourceType.S3:
        return createS3Source(source.id, source.name, config as S3Configuration);
      
      case DataSourceType.HTTP:
        return createHTTPSource(source.id, source.name, config as HTTPConfiguration);
      
      case DataSourceType.FILESYSTEM:
        return createFilesystemSource(source.id, source.name, config as FilesystemConfiguration);
      
      default:
        throw new Error(`Unknown source type: ${source.type}`);
    }
  }
}
import { getQueueForSource, isLocalSource } from '../queue-utils';
import { Source } from '@/lib/domain/source/entities/source';
import { DataSourceType } from '@/generated/prisma';

describe('queue-utils', () => {
  describe('getQueueForSource', () => {
    it('should return source-specific queue for S3 sources', () => {
      const source = {
        id: 's3-source-id',
        name: 's3-source',
        type: DataSourceType.S3,
        configuration: {},
        enabled: true,
        createdAt: new Date(),
        updatedAt: new Date(),
      } as Source;
      
      expect(getQueueForSource(source)).toBe('source.s3-source-id');
    });
    
    it('should return source-specific queue for HTTP sources', () => {
      const source = {
        id: 'http-source-id',
        name: 'http-source',
        type: DataSourceType.HTTP,
        configuration: {},
        enabled: true,
        createdAt: new Date(),
        updatedAt: new Date(),
      } as Source;
      
      expect(getQueueForSource(source)).toBe('source.http-source-id');
    });
    
    it('should return source-specific queue for filesystem sources', () => {
      const source = {
        id: 'fs-source-id',
        name: 'local-source',
        type: DataSourceType.FILESYSTEM,
        configuration: {},
        enabled: true,
        createdAt: new Date(),
        updatedAt: new Date(),
      } as Source;
      
      expect(getQueueForSource(source)).toBe('source.fs-source-id');
    });
    
    it('should return source-specific queue for worker sources', () => {
      const source = {
        id: 'worker-123',
        name: 'worker-hostname',
        type: DataSourceType.FILESYSTEM,
        configuration: {},
        enabled: true,
        createdAt: new Date(),
        updatedAt: new Date(),
      } as Source;
      
      expect(getQueueForSource(source)).toBe('source.worker-123');
    });
  });
  
  describe('isLocalSource', () => {
    it('should return true for filesystem sources', () => {
      const source = {
        id: 'test-id',
        name: 'local-source',
        type: DataSourceType.FILESYSTEM,
        configuration: {},
        enabled: true,
        createdAt: new Date(),
        updatedAt: new Date(),
      } as Source;
      
      expect(isLocalSource(source)).toBe(true);
    });
    
    it('should return false for non-filesystem sources', () => {
      const s3Source = {
        id: 'test-id',
        name: 's3-source',
        type: DataSourceType.S3,
        configuration: {},
        enabled: true,
        createdAt: new Date(),
        updatedAt: new Date(),
      } as Source;
      
      const httpSource = {
        id: 'test-id',
        name: 'http-source',
        type: DataSourceType.HTTP,
        configuration: {},
        enabled: true,
        createdAt: new Date(),
        updatedAt: new Date(),
      } as Source;
      
      expect(isLocalSource(s3Source)).toBe(false);
      expect(isLocalSource(httpSource)).toBe(false);
    });
  });
});
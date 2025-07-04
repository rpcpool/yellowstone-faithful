import { SourceDomainService } from '@/lib/domain/source/services/source-domain-service';
import { SourceRepository } from '@/lib/domain/source/repositories/source-repository';
import { Source } from '@/lib/domain/source/entities/source';
import { DataSourceType } from '@/generated/prisma';

describe('SourceDomainService', () => {
  let service: SourceDomainService;
  let mockRepository: jest.Mocked<SourceRepository>;

  beforeEach(() => {
    mockRepository = {
      findById: jest.fn(),
      findByName: jest.fn(),
      findAll: jest.fn(),
      save: jest.fn(),
      delete: jest.fn()
    };
    
    service = new SourceDomainService(mockRepository);
  });

  describe('createSource', () => {
    describe('FILESYSTEM type validation', () => {
      it('should create a FILESYSTEM source with valid configuration', async () => {
        const name = 'test-filesystem';
        const type = DataSourceType.FILESYSTEM;
        const configuration = {
          basePath: '/path/to/data'
        };

        mockRepository.findByName.mockResolvedValue(null);
        mockRepository.save.mockResolvedValue(undefined);

        const result = await service.createSource(name, type, configuration);

        expect(result).toBeInstanceOf(Source);
        expect(result.name).toBe(name);
        expect(result.type).toBe(type);
        expect(result.configuration).toEqual(configuration);
        expect(mockRepository.save).toHaveBeenCalled();
      });

      it('should throw error for FILESYSTEM source without basePath', async () => {
        const name = 'test-filesystem';
        const type = DataSourceType.FILESYSTEM;
        const configuration = {};

        mockRepository.findByName.mockResolvedValue(null);

        await expect(
          service.createSource(name, type, configuration)
        ).rejects.toThrow('Filesystem source requires basePath');
      });

      it('should create a worker source (FILESYSTEM) with extended configuration', async () => {
        const name = 'worker-test-host';
        const type = DataSourceType.FILESYSTEM;
        const configuration = {
          basePath: '/workers/test-host',
          isWorker: true,
          hostname: 'test-host',
          pid: 12345,
          capabilities: ['default', 'local'],
          startedAt: new Date().toISOString()
        };

        mockRepository.findByName.mockResolvedValue(null);
        mockRepository.save.mockResolvedValue(undefined);

        const result = await service.createSource(name, type, configuration);

        expect(result).toBeInstanceOf(Source);
        expect(result.configuration).toEqual(configuration);
        expect(mockRepository.save).toHaveBeenCalled();
      });
    });

    describe('Duplicate name validation', () => {
      it('should throw error if source with same name already exists', async () => {
        const existingSource = new Source(
          'existing-id',
          'worker-test-host',
          DataSourceType.FILESYSTEM,
          { basePath: '/workers/test-host' },
          true,
          new Date(),
          new Date()
        );

        mockRepository.findByName.mockResolvedValue(existingSource);

        await expect(
          service.createSource('worker-test-host', DataSourceType.FILESYSTEM, { basePath: '/path' })
        ).rejects.toThrow('Source with name "worker-test-host" already exists');
      });
    });
  });

  describe('updateSource', () => {
    describe('Configuration updates', () => {
      it('should update source configuration while preserving existing fields', async () => {
        const existingSource = new Source(
          'worker-123',
          'worker-test-host',
          DataSourceType.FILESYSTEM,
          {
            basePath: '/workers/test-host',
            isWorker: true,
            hostname: 'test-host',
            pid: 12345,
            capabilities: ['default', 'local'],
            startedAt: '2024-01-01T00:00:00Z'
          },
          true,
          new Date(),
          new Date()
        );

        const newConfiguration = {
          basePath: '/workers/test-host',
          isWorker: true,
          hostname: 'test-host',
          pid: 67890, // New PID
          capabilities: ['default', 'local', 'heavy'], // Updated capabilities
          startedAt: '2024-01-02T00:00:00Z' // New start time
        };

        mockRepository.findById.mockResolvedValue(existingSource);
        mockRepository.save.mockResolvedValue(undefined);

        const result = await service.updateSource('worker-123', {
          configuration: newConfiguration
        });

        expect(result.configuration).toEqual(newConfiguration);
        expect(mockRepository.save).toHaveBeenCalled();
      });

      it('should validate configuration based on source type during update', async () => {
        const existingSource = new Source(
          'fs-123',
          'test-filesystem',
          DataSourceType.FILESYSTEM,
          { basePath: '/original/path' },
          true,
          new Date(),
          new Date()
        );

        mockRepository.findById.mockResolvedValue(existingSource);

        await expect(
          service.updateSource('fs-123', {
            configuration: {} // Missing basePath
          })
        ).rejects.toThrow('Filesystem source requires basePath');
      });
    });

    describe('Worker-specific updates', () => {
      it('should handle worker re-registration with new PID', async () => {
        const existingWorker = new Source(
          'worker-456',
          'worker-prod-host',
          DataSourceType.FILESYSTEM,
          {
            basePath: '/workers/prod-host',
            isWorker: true,
            hostname: 'prod-host',
            pid: 11111,
            capabilities: ['default'],
            startedAt: '2024-01-01T00:00:00Z'
          },
          true,
          new Date('2024-01-01'),
          new Date('2024-01-01')
        );

        const updatedConfiguration = {
          basePath: '/workers/prod-host',
          isWorker: true,
          hostname: 'prod-host',
          pid: 22222, // New PID after restart
          capabilities: ['default', 'local'],
          startedAt: '2024-01-02T00:00:00Z'
        };

        mockRepository.findById.mockResolvedValue(existingWorker);
        mockRepository.save.mockResolvedValue(undefined);

        const result = await service.updateSource('worker-456', {
          configuration: updatedConfiguration
        });

        expect(result.configuration.pid).toBe(22222);
        expect(result.configuration.startedAt).toBe('2024-01-02T00:00:00Z');
        expect(result.name).toBe('worker-prod-host'); // Name unchanged
        expect(result.id).toBe('worker-456'); // ID unchanged
      });
    });
  });

  describe('S3 source validation', () => {
    it('should create S3 source with valid configuration', async () => {
      const configuration = {
        bucket: 'test-bucket',
        region: 'us-east-1',
        endpoint: 'https://s3.amazonaws.com'
      };

      mockRepository.findByName.mockResolvedValue(null);
      mockRepository.save.mockResolvedValue(undefined);

      const result = await service.createSource('test-s3', DataSourceType.S3, configuration);

      expect(result.configuration).toEqual(configuration);
    });

    it('should throw error for S3 source without required fields', async () => {
      mockRepository.findByName.mockResolvedValue(null);

      await expect(
        service.createSource('test-s3', DataSourceType.S3, { bucket: 'test' })
      ).rejects.toThrow('S3 source requires bucket and region');
    });
  });

  describe('HTTP source validation', () => {
    it('should create HTTP source with valid configuration', async () => {
      const configuration = {
        host: 'https://example.com',
        path: '/data',
        headers: { 'Authorization': 'Bearer token' }
      };

      mockRepository.findByName.mockResolvedValue(null);
      mockRepository.save.mockResolvedValue(undefined);

      const result = await service.createSource('test-http', DataSourceType.HTTP, configuration);

      expect(result.configuration).toEqual(configuration);
    });

    it('should throw error for HTTP source without required fields', async () => {
      mockRepository.findByName.mockResolvedValue(null);

      await expect(
        service.createSource('test-http', DataSourceType.HTTP, { host: 'https://example.com' })
      ).rejects.toThrow('HTTP source requires host and path');
    });
  });
});
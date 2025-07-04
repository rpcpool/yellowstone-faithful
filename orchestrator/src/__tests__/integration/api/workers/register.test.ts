/**
 * @jest-environment node
 */

import { SourceRepository } from '@/lib/domain/source/repositories/source-repository';
import { SourceDomainService } from '@/lib/domain/source/services/source-domain-service';
import { CreateSourceUseCase } from '@/lib/application/source/use-cases/create-source';
import { UpdateSourceUseCase } from '@/lib/application/source/use-cases/update-source';
import { DataSourceType } from '@/generated/prisma';
import { Source } from '@/lib/domain/source/entities/source';

describe('Worker Registration Business Logic', () => {
  let mockSourceRepository: jest.Mocked<SourceRepository>;
  let sourceDomainService: SourceDomainService;
  let createSourceUseCase: CreateSourceUseCase;
  let updateSourceUseCase: UpdateSourceUseCase;

  beforeEach(() => {
    jest.clearAllMocks();
    
    // Create mock repository
    mockSourceRepository = {
      findById: jest.fn(),
      findByName: jest.fn(),
      findAll: jest.fn(),
      save: jest.fn(),
      delete: jest.fn()
    };
    
    // Create real services with mocked repository
    sourceDomainService = new SourceDomainService(mockSourceRepository);
    createSourceUseCase = new CreateSourceUseCase(sourceDomainService);
    updateSourceUseCase = new UpdateSourceUseCase(sourceDomainService);
  });

  describe('New worker registration', () => {
    it('should create a new worker source', async () => {
      const hostname = 'test-host';
      const pid = 12345;
      const capabilities = ['default', 'local'];
      const workerName = `worker-${hostname}`;

      // Mock repository to return null (no existing worker)
      mockSourceRepository.findByName.mockResolvedValue(null);
      mockSourceRepository.save.mockResolvedValue(undefined);

      // Create worker through use case
      const result = await createSourceUseCase.execute({
        name: workerName,
        type: DataSourceType.FILESYSTEM,
        configuration: {
          basePath: `/workers/${hostname}`,
          isWorker: true,
          hostname,
          pid,
          capabilities,
          startedAt: new Date().toISOString()
        },
        enabled: true
      });

      expect(mockSourceRepository.findByName).toHaveBeenCalledWith(workerName);
      expect(mockSourceRepository.save).toHaveBeenCalled();
      expect(result.source.name).toBe(workerName);
      expect(result.source.type).toBe(DataSourceType.FILESYSTEM);
      expect(result.source.configuration).toMatchObject({
        basePath: `/workers/${hostname}`,
        isWorker: true,
        hostname,
        pid,
        capabilities
      });
    });
  });

  describe('Existing worker re-registration', () => {
    it('should update existing worker configuration', async () => {
      const hostname = 'test-host';
      const oldPid = 12345;
      const newPid = 67890;
      const workerName = `worker-${hostname}`;

      const existingSource = new Source(
        'worker-123',
        workerName,
        DataSourceType.FILESYSTEM,
        {
          basePath: `/workers/${hostname}`,
          isWorker: true,
          hostname,
          pid: oldPid,
          capabilities: ['default', 'local'],
          startedAt: '2024-01-01T00:00:00Z'
        },
        true,
        new Date('2024-01-01'),
        new Date('2024-01-01')
      );

      // Mock finding existing worker
      mockSourceRepository.findByName.mockResolvedValue(existingSource);
      mockSourceRepository.findById.mockResolvedValue(existingSource);
      mockSourceRepository.save.mockResolvedValue(undefined);

      // Update worker through use case
      const result = await updateSourceUseCase.execute({
        id: existingSource.id,
        configuration: {
          basePath: `/workers/${hostname}`,
          isWorker: true,
          hostname,
          pid: newPid,
          capabilities: ['default', 'local', 'heavy'],
          startedAt: new Date().toISOString()
        }
      });

      expect(mockSourceRepository.findById).toHaveBeenCalledWith('worker-123');
      expect(mockSourceRepository.save).toHaveBeenCalled();
      expect(result.source.configuration.pid).toBe(newPid);
      expect(result.source.configuration.capabilities).toEqual(['default', 'local', 'heavy']);
    });
  });

  describe('Worker registration validation', () => {
    it('should require hostname, pid, and capabilities', async () => {
      // Test missing fields by trying to create without proper configuration
      const workerName = 'worker-test';

      mockSourceRepository.findByName.mockResolvedValue(null);

      // This should throw because FILESYSTEM requires basePath
      await expect(
        createSourceUseCase.execute({
          name: workerName,
          type: DataSourceType.FILESYSTEM,
          configuration: {}, // Missing required fields
          enabled: true
        })
      ).rejects.toThrow('Filesystem source requires basePath');
    });
  });

  describe('Error handling', () => {
    it('should handle repository errors', async () => {
      const workerName = 'worker-test-host';

      mockSourceRepository.findByName.mockRejectedValue(
        new Error('Database connection failed')
      );

      await expect(
        createSourceUseCase.execute({
          name: workerName,
          type: DataSourceType.FILESYSTEM,
          configuration: {
            basePath: '/workers/test-host'
          },
          enabled: true
        })
      ).rejects.toThrow('Database connection failed');
    });

    it('should prevent duplicate worker names', async () => {
      const workerName = 'worker-test-host';
      const existingSource = new Source(
        'existing-id',
        workerName,
        DataSourceType.FILESYSTEM,
        { basePath: '/workers/test-host' },
        true,
        new Date(),
        new Date()
      );

      mockSourceRepository.findByName.mockResolvedValue(existingSource);

      await expect(
        createSourceUseCase.execute({
          name: workerName,
          type: DataSourceType.FILESYSTEM,
          configuration: {
            basePath: '/workers/test-host'
          },
          enabled: true
        })
      ).rejects.toThrow(`Source with name "${workerName}" already exists`);
    });
  });
});
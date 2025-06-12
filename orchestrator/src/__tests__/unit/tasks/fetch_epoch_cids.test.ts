import * as epochsModule from '@/lib/epochs';
import * as faktoryModule from '@/lib/infrastructure/faktory/faktory-client';
import fetchEpochCidsTask from '@/lib/tasks/fetch_epoch_cids';
import { prisma } from '@/lib/infrastructure/persistence/prisma';

// Mock dependencies
jest.mock('@/lib/epochs');
jest.mock('@/lib/infrastructure/faktory/faktory-client');
jest.mock('@/lib/infrastructure/persistence/prisma', () => ({
  prisma: {
    epoch: {
      update: jest.fn(),
    },
  },
}));

// Mock fetch
const fetchMock = jest.fn();
global.fetch = fetchMock;
globalThis.fetch = fetchMock;

// Helper for fetch mock response
function makeFetchResponse(ok: boolean, textValue?: string) {
  return {
    ok,
    text: jest.fn().mockResolvedValue(textValue ?? ''),
  };
}

describe('fetchEpochCidsTask', () => {
  const mockGetLatestEpoch = epochsModule.getLatestEpoch as jest.Mock;
  const mockGetEpochs = epochsModule.getEpochs as jest.Mock;
  const mockPrismaUpdate = prisma.epoch.update as jest.Mock;
  const mockFetch = fetchMock as jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    console.error = jest.fn();
    console.log = jest.fn();
    mockPrismaUpdate.mockResolvedValue(undefined);
  });

  describe('validateArgs', () => {
    it('should validate empty object as valid args', () => {
      expect(fetchEpochCidsTask.validateArgs({})).toBe(true);
    });

    it('should reject non-object args', () => {
      expect(fetchEpochCidsTask.validateArgs('string')).toBe(false);
      expect(fetchEpochCidsTask.validateArgs(123)).toBe(false);
      expect(fetchEpochCidsTask.validateArgs(null)).toBe(false);
    });
  });

  describe('run', () => {
    it('should fetch CIDs for all epochs successfully', async () => {
      mockGetLatestEpoch.mockResolvedValue(2);
      mockGetEpochs.mockResolvedValue([
        { id: 1, epoch: 0 },
        { id: 2, epoch: 1 },
        { id: 3, epoch: 2 },
      ]);

      mockFetch
        .mockResolvedValueOnce(makeFetchResponse(true, 'cid-epoch-0\n'))
        .mockResolvedValueOnce(makeFetchResponse(true, 'cid-epoch-1\n'))
        .mockResolvedValueOnce(makeFetchResponse(true, 'cid-epoch-2\n'));

      let result;
      try {
        result = await fetchEpochCidsTask.run({});
      } catch (e) {
        console.error('Test error (should fetch CIDs for all epochs successfully):', e);
        throw e;
      }

      expect(result).toBe(true);
      expect(mockGetLatestEpoch).toHaveBeenCalled();
      expect(mockGetEpochs).toHaveBeenCalledWith(0, 2);

      expect(mockFetch).toHaveBeenCalledTimes(3);
      expect(mockFetch).toHaveBeenCalledWith('https://files.old-faithful.net/0/epoch-0.cid');
      expect(mockFetch).toHaveBeenCalledWith('https://files.old-faithful.net/1/epoch-1.cid');
      expect(mockFetch).toHaveBeenCalledWith('https://files.old-faithful.net/2/epoch-2.cid');

      expect(mockPrismaUpdate).toHaveBeenCalledTimes(3);
      expect(mockPrismaUpdate).toHaveBeenCalledWith({
        where: { id: 1 },
        data: { cid: 'cid-epoch-0' },
      });
      expect(mockPrismaUpdate).toHaveBeenCalledWith({
        where: { id: 2 },
        data: { cid: 'cid-epoch-1' },
      });
      expect(mockPrismaUpdate).toHaveBeenCalledWith({
        where: { id: 3 },
        data: { cid: 'cid-epoch-2' },
      });
    });

    it('should handle failed CID fetches gracefully', async () => {
      mockGetLatestEpoch.mockResolvedValue(1);
      mockGetEpochs.mockResolvedValue([
        { id: 1, epoch: 0 },
        { id: 2, epoch: 1 },
      ]);

      mockFetch
        .mockResolvedValueOnce(makeFetchResponse(false))
        .mockResolvedValueOnce(makeFetchResponse(true, 'cid-epoch-1'));

      let result;
      try {
        result = await fetchEpochCidsTask.run({});
      } catch (e) {
        console.error('Test error (should handle failed CID fetches gracefully):', e);
        throw e;
      }

      expect(result).toBe(true);
      expect(console.error).toHaveBeenCalledWith('Failed to fetch CID for epoch 0');
      
      // Only successful fetch should update database
      expect(mockPrismaUpdate).toHaveBeenCalledTimes(1);
      expect(mockPrismaUpdate).toHaveBeenCalledWith({
        where: { id: 2 },
        data: { cid: 'cid-epoch-1' },
      });
    });

    it('should trim whitespace from CID text', async () => {
      mockGetLatestEpoch.mockResolvedValue(0);
      mockGetEpochs.mockResolvedValue([{ id: 1, epoch: 0 }]);

      mockFetch.mockResolvedValueOnce(makeFetchResponse(true, '  cid-with-spaces  \n\t'));

      try {
        await fetchEpochCidsTask.run({});
      } catch (e) {
        console.error('Test error (should trim whitespace from CID text):', e);
        throw e;
      }

      expect(mockPrismaUpdate).toHaveBeenCalledWith({
        where: { id: 1 },
        data: { cid: 'cid-with-spaces' },
      });
    });

    it('should handle errors and return false', async () => {
      mockGetLatestEpoch.mockRejectedValue(new Error('Database error'));

      const result = await fetchEpochCidsTask.run({});

      expect(result).toBe(false);
      expect(console.error).toHaveBeenCalledWith(
        'Error in fetchEpochCidsTask:',
        expect.any(Error)
      );
    });

    it('should handle empty epochs list', async () => {
      mockGetLatestEpoch.mockResolvedValue(0);
      mockGetEpochs.mockResolvedValue([]);

      const result = await fetchEpochCidsTask.run({});

      expect(result).toBe(true);
      expect(mockFetch).not.toHaveBeenCalled();
      expect(mockPrismaUpdate).not.toHaveBeenCalled();
    });
  });

  describe('schedule', () => {
    it('should schedule job correctly', async () => {
      const mockJob = {
        queue: '',
        reserveFor: 0,
        push: jest.fn(),
        jid: 'test-job-id',
      };

      const mockClient = {
        job: jest.fn().mockReturnValue(mockJob),
      };

      (faktoryModule as { client: unknown }).client = mockClient;

      const jobId = await fetchEpochCidsTask.schedule({});

      expect(mockClient.job).toHaveBeenCalledWith('fetchEpochCids', {});
      expect(mockJob.queue).toBe('default');
      expect(mockJob.reserveFor).toBe(1000);
      expect(mockJob.push).toHaveBeenCalled();
      expect(jobId).toBe('test-job-id');
    });
  });
});
import { checkEpochExists } from '@/lib/epochs/check-epoch-exists';
import * as runOnAllSourcesModule from '@/lib/epochs/run-on-all-sources';

jest.mock('@/lib/epochs/run-on-all-sources');

describe('checkEpochExists', () => {
  const mockRunOnAllSources = runOnAllSourcesModule.runOnAllSources as jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('should return exists true when epoch exists in sources', async () => {
    mockRunOnAllSources.mockResolvedValueOnce([
      { source: 'source1', result: true },
      { source: 'source2', result: true },
      { source: 'source3', result: false },
    ]);

    const results = await checkEpochExists(123);

    expect(mockRunOnAllSources).toHaveBeenCalledWith(
      'checkEpochExists',
      123,
      expect.any(Function)
    );

    expect(results).toEqual([
      { source: 'source1', exists: true },
      { source: 'source2', exists: true },
      { source: 'source3', exists: false },
    ]);
  });

  it('should handle null results as false', async () => {
    mockRunOnAllSources.mockResolvedValueOnce([
      { source: 'source1', result: true },
      { source: 'source2', result: null },
      { source: 'source3', result: undefined },
    ]);

    const results = await checkEpochExists(456);

    expect(results).toEqual([
      { source: 'source1', exists: true },
      { source: 'source2', exists: false },
      { source: 'source3', exists: false },
    ]);
  });

  it('should handle empty sources', async () => {
    mockRunOnAllSources.mockResolvedValueOnce([]);

    const results = await checkEpochExists(789);

    expect(results).toEqual([]);
  });

  it('should pass correct function to runOnAllSources', async () => {
    mockRunOnAllSources.mockImplementationOnce(async (operation, epochId, fn) => {
      const mockSource = {
        epochExists: jest.fn().mockResolvedValue(true),
      };
      await fn(mockSource);
      expect(mockSource.epochExists).toHaveBeenCalledWith(epochId);
      return [{ source: 'test', result: true }];
    });

    await checkEpochExists(999);

    expect(mockRunOnAllSources).toHaveBeenCalledWith(
      'checkEpochExists',
      999,
      expect.any(Function)
    );
  });

  it('should handle errors from runOnAllSources', async () => {
    mockRunOnAllSources.mockRejectedValueOnce(new Error('Source error'));

    await expect(checkEpochExists(123)).rejects.toThrow('Source error');
  });
});
import { getEpochInfo, EpochInfoResponse } from '@/lib/solana';

// Mock fetch
global.fetch = jest.fn();

describe('Solana utilities', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('getEpochInfo', () => {
    it('should fetch epoch info successfully', async () => {
      const mockResponse: EpochInfoResponse = {
        jsonrpc: '2.0',
        result: {
          absoluteSlot: 123456789,
          blockHeight: 123456000,
          epoch: 289,
          slotIndex: 123456,
          slotsInEpoch: 432000,
          transactionCount: 987654321,
        },
        id: 1,
      };

      (fetch as jest.Mock).mockResolvedValueOnce({
        json: jest.fn().mockResolvedValueOnce(mockResponse),
      });

      const result = await getEpochInfo();

      expect(fetch).toHaveBeenCalledWith('https://api.mainnet-beta.solana.com', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'getEpochInfo' }),
      });

      expect(result).toEqual(mockResponse);
      expect(result.result.epoch).toBe(289);
      expect(result.result.slotsInEpoch).toBe(432000);
    });

    it('should handle fetch errors', async () => {
      (fetch as jest.Mock).mockRejectedValueOnce(new Error('Network error'));

      await expect(getEpochInfo()).rejects.toThrow('Network error');
    });

    it('should handle JSON parsing errors', async () => {
      (fetch as jest.Mock).mockResolvedValueOnce({
        json: jest.fn().mockRejectedValueOnce(new Error('Invalid JSON')),
      });

      await expect(getEpochInfo()).rejects.toThrow('Invalid JSON');
    });

    it('should make correct RPC call', async () => {
      const mockResponse: EpochInfoResponse = {
        jsonrpc: '2.0',
        result: {
          absoluteSlot: 0,
          blockHeight: 0,
          epoch: 0,
          slotIndex: 0,
          slotsInEpoch: 432000,
          transactionCount: 0,
        },
        id: 1,
      };

      (fetch as jest.Mock).mockResolvedValueOnce({
        json: jest.fn().mockResolvedValueOnce(mockResponse),
      });

      await getEpochInfo();

      const [[url, options]] = (fetch as jest.Mock).mock.calls;
      const body = JSON.parse(options.body);

      expect(url).toBe('https://api.mainnet-beta.solana.com');
      expect(options.method).toBe('POST');
      expect(options.headers['Content-Type']).toBe('application/json');
      expect(body.jsonrpc).toBe('2.0');
      expect(body.method).toBe('getEpochInfo');
      expect(body.id).toBe(1);
    });
  });
});
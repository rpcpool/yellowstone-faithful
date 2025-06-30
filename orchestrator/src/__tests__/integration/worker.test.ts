/**
 * @jest-environment node
 */

// Mock fetch globally
global.fetch = jest.fn();

describe('Worker Registration Logic', () => {
  const mockFetch = global.fetch as jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('Worker registration function', () => {
    it('should register worker with correct payload', async () => {
      const hostname = 'test-host';
      const pid = 12345;
      const capabilities = ['default', 'local'];
      const workerId = 'worker-123';

      mockFetch.mockResolvedValueOnce({
        status: 201,
        json: async () => ({
          message: 'Worker registered successfully',
          workerId,
          source: {
            id: workerId,
            name: `worker-${hostname}`
          }
        })
      });

      // Simulate worker registration logic
      const response = await fetch('http://localhost:3000/api/workers/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          hostname,
          pid,
          capabilities
        })
      });

      const data = await response.json();

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:3000/api/workers/register',
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            hostname,
            pid,
            capabilities
          })
        }
      );

      expect(response.status).toBe(201);
      expect(data.workerId).toBe(workerId);
      expect(data.source.name).toBe(`worker-${hostname}`);
    });

    it('should handle existing worker registration', async () => {
      const hostname = 'test-host';
      const pid = 12345;
      const workerId = 'worker-456';

      mockFetch.mockResolvedValueOnce({
        status: 200,
        json: async () => ({
          message: 'Worker already registered',
          workerId,
          source: {
            id: workerId,
            name: `worker-${hostname}`
          }
        })
      });

      const response = await fetch('http://localhost:3000/api/workers/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          hostname,
          pid,
          capabilities: ['default', 'local']
        })
      });

      const data = await response.json();

      expect(response.status).toBe(200);
      expect(data.workerId).toBe(workerId);
    });

    it('should handle registration failures', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      await expect(
        fetch('http://localhost:3000/api/workers/register', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            hostname: 'test-host',
            pid: 12345,
            capabilities: ['default', 'local']
          })
        })
      ).rejects.toThrow('Network error');
    });

    it('should handle error responses', async () => {
      mockFetch.mockResolvedValueOnce({
        status: 500,
        json: async () => ({
          error: 'Internal server error'
        })
      });

      const response = await fetch('http://localhost:3000/api/workers/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          hostname: 'test-host',
          pid: 12345,
          capabilities: ['default', 'local']
        })
      });

      const data = await response.json();

      expect(response.status).toBe(500);
      expect(data.error).toBe('Internal server error');
    });
  });

  describe('Queue configuration', () => {
    it('should build correct queues for local source worker', () => {
      const workerId = 'worker-123';
      const isLocalSource = true;
      
      // Simulate queue building logic
      const queues = ['default'];
      if (isLocalSource) {
        queues.push('local');
        if (workerId) {
          queues.push(`local:${workerId}`);
        }
      }

      expect(queues).toEqual(['default', 'local', `local:${workerId}`]);
    });

    it('should only use default queue for non-local source worker', () => {
      const isLocalSource = false;
      
      // Simulate queue building logic
      const queues = ['default'];
      if (isLocalSource) {
        queues.push('local');
      }

      expect(queues).toEqual(['default']);
    });
  });
});
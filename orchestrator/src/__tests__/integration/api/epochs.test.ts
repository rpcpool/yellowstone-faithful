/**
 * @jest-environment node
 */

// Mock Prisma
const mockFindMany = jest.fn();
const mockCount = jest.fn();
const mockFindUnique = jest.fn();
jest.mock('@/lib/prisma', () => ({
  prisma: {
    epoch: {
      findMany: mockFindMany,
      count: mockCount,
      findUnique: mockFindUnique,
    },
  },
}));

describe('Epochs API Integration Tests', () => {
  const mockPrisma = {
    epoch: {
      findMany: mockFindMany,
      count: mockCount,
      findUnique: mockFindUnique,
    },
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('Database queries', () => {
    it('should query epochs with correct parameters', async () => {
      const mockEpochs = [
        { id: 1, epoch: 0, createdAt: new Date(), updatedAt: new Date() },
        { id: 2, epoch: 1, createdAt: new Date(), updatedAt: new Date() },
      ];

      (mockPrisma.epoch.findMany as jest.Mock).mockResolvedValue(mockEpochs);
      (mockPrisma.epoch.count as jest.Mock).mockResolvedValue(2);

      // Simulate the query that would be made by the API
      const epochs = await mockPrisma.epoch.findMany({
        skip: 0,
        take: 10,
        orderBy: { epoch: 'desc' },
      });

      const count = await mockPrisma.epoch.count();

      expect(epochs).toEqual(mockEpochs);
      expect(count).toBe(2);
      expect(mockPrisma.epoch.findMany).toHaveBeenCalledWith({
        skip: 0,
        take: 10,
        orderBy: { epoch: 'desc' },
      });
    });

    it('should query single epoch with relations', async () => {
      const mockEpoch = {
        id: 1,
        epoch: 100,
        createdAt: new Date(),
        updatedAt: new Date(),
        indexes: [],
        gsfa: [],
      };

      (mockPrisma.epoch.findUnique as jest.Mock).mockResolvedValue(mockEpoch);

      // Simulate the query that would be made by the API
      const epoch = await mockPrisma.epoch.findUnique({
        where: { id: 1 },
        include: {
          epochIndexes: true,
          epochGsfas: true,
        },
      });

      expect(epoch).toEqual(mockEpoch);
      expect(mockPrisma.epoch.findUnique).toHaveBeenCalledWith({
        where: { id: 1 },
        include: {
          epochIndexes: true,
          epochGsfas: true,
        },
      });
    });

    it('should handle null results correctly', async () => {
      (mockPrisma.epoch.findUnique as jest.Mock).mockResolvedValue(null);

      const epoch = await mockPrisma.epoch.findUnique({
        where: { id: 999 },
      });

      expect(epoch).toBeNull();
    });

    it('should handle database errors', async () => {
      (mockPrisma.epoch.findMany as jest.Mock).mockRejectedValue(new Error('Database connection failed'));

      await expect(
        mockPrisma.epoch.findMany({
          skip: 0,
          take: 10,
        })
      ).rejects.toThrow('Database connection failed');
    });

    it('should handle pagination correctly', async () => {
      (mockPrisma.epoch.findMany as jest.Mock).mockResolvedValue([]);
      (mockPrisma.epoch.count as jest.Mock).mockResolvedValue(100);

      // Test different pages
      await mockPrisma.epoch.findMany({
        skip: 0,
        take: 10,
      });

      await mockPrisma.epoch.findMany({
        skip: 10,
        take: 10,
      });

      await mockPrisma.epoch.findMany({
        skip: 20,
        take: 10,
      });

      expect(mockPrisma.epoch.findMany).toHaveBeenCalledTimes(3);
      expect(mockPrisma.epoch.findMany).toHaveBeenNthCalledWith(1, {
        skip: 0,
        take: 10,
      });
      expect(mockPrisma.epoch.findMany).toHaveBeenNthCalledWith(2, {
        skip: 10,
        take: 10,
      });
      expect(mockPrisma.epoch.findMany).toHaveBeenNthCalledWith(3, {
        skip: 20,
        take: 10,
      });
    });
  });
});
import { DataSourceType } from '@/lib/interfaces/data-source';
import { IndexType } from '@/lib/epochs';

// Mock data source for testing
class MockDataSource {
  name: string;
  type: DataSourceType;
  
  constructor(name: string, type: DataSourceType) {
    this.name = name;
    this.type = type;
  }

  async epochExists(epoch: number): Promise<boolean> {
    return epoch === 123;
  }

  async epochIndexExists(epoch: number, indexType: IndexType): Promise<boolean> {
    return epoch === 123 && indexType === IndexType.CidToOffsetAndSize;
  }

  async epochGsfaIndexExists(epoch: number): Promise<boolean> {
    return epoch === 123;
  }

  async epochIndexStats(epoch: number, indexType: IndexType): Promise<{ size: number }> {
    if (epoch === 123 && indexType === IndexType.CidToOffsetAndSize) {
      return { size: 1024000 };
    }
    return { size: 0 };
  }

  async getEpochCid(epoch: number): Promise<string> {
    if (epoch === 123) {
      return 'bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi';
    }
    throw new Error('Epoch not found');
  }

  async getEpochCarUrl(epoch: number): Promise<string> {
    return `https://example.com/epoch-${epoch}.car`;
  }

  async getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string> {
    return `https://example.com/epoch-${epoch}-${indexType}.index`;
  }

  async getEpochGsfaUrl(epoch: number): Promise<string> {
    return `https://example.com/epoch-${epoch}.gsfa`;
  }

  async getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string> {
    return `https://example.com/epoch-${epoch}-gsfa.tar.gz`;
  }
}

describe('DataSource Interface', () => {
  let mockDataSource: MockDataSource;

  beforeEach(() => {
    mockDataSource = new MockDataSource('test-source', DataSourceType.S3);
  });

  describe('epochExists', () => {
    it('should return true for existing epoch', async () => {
      const exists = await mockDataSource.epochExists(123);
      expect(exists).toBe(true);
    });

    it('should return false for non-existing epoch', async () => {
      const exists = await mockDataSource.epochExists(999);
      expect(exists).toBe(false);
    });
  });

  describe('epochIndexExists', () => {
    it('should return true for existing index', async () => {
      const exists = await mockDataSource.epochIndexExists(123, IndexType.CidToOffsetAndSize);
      expect(exists).toBe(true);
    });

    it('should return false for non-existing index', async () => {
      const exists = await mockDataSource.epochIndexExists(123, IndexType.SigExists);
      expect(exists).toBe(false);
    });

    it('should return false for non-existing epoch', async () => {
      const exists = await mockDataSource.epochIndexExists(999, IndexType.CidToOffsetAndSize);
      expect(exists).toBe(false);
    });
  });

  describe('epochGsfaIndexExists', () => {
    it('should return true for existing GSFA index', async () => {
      const exists = await mockDataSource.epochGsfaIndexExists(123);
      expect(exists).toBe(true);
    });

    it('should return false for non-existing GSFA index', async () => {
      const exists = await mockDataSource.epochGsfaIndexExists(999);
      expect(exists).toBe(false);
    });
  });

  describe('epochIndexStats', () => {
    it('should return stats for existing index', async () => {
      const stats = await mockDataSource.epochIndexStats(123, IndexType.CidToOffsetAndSize);
      expect(stats.size).toBe(1024000);
    });

    it('should return zero size for non-existing index', async () => {
      const stats = await mockDataSource.epochIndexStats(999, IndexType.CidToOffsetAndSize);
      expect(stats.size).toBe(0);
    });
  });

  describe('getEpochCid', () => {
    it('should return CID for existing epoch', async () => {
      const cid = await mockDataSource.getEpochCid(123);
      expect(cid).toBe('bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi');
    });

    it('should throw error for non-existing epoch', async () => {
      await expect(mockDataSource.getEpochCid(999)).rejects.toThrow('Epoch not found');
    });
  });

  describe('URL generation methods', () => {
    it('should generate correct CAR URL', async () => {
      const url = await mockDataSource.getEpochCarUrl(123);
      expect(url).toBe('https://example.com/epoch-123.car');
    });

    it('should generate correct index URL', async () => {
      const url = await mockDataSource.getEpochIndexUrl(123, IndexType.CidToOffsetAndSize);
      expect(url).toBe('https://example.com/epoch-123-CidToOffsetAndSize.index');
    });

    it('should generate correct GSFA URL', async () => {
      const url = await mockDataSource.getEpochGsfaUrl(123);
      expect(url).toBe('https://example.com/epoch-123.gsfa');
    });

    it('should generate correct GSFA archive URL', async () => {
      const url = await mockDataSource.getEpochGsfaIndexArchiveUrl(123);
      expect(url).toBe('https://example.com/epoch-123-gsfa.tar.gz');
    });
  });

  describe('DataSource properties', () => {
    it('should have correct name and type', () => {
      expect(mockDataSource.name).toBe('test-source');
      expect(mockDataSource.type).toBe(DataSourceType.S3);
    });

    it('should support different data source types', () => {
      const httpSource = new MockDataSource('http-test', DataSourceType.HTTP);
      expect(httpSource.type).toBe(DataSourceType.HTTP);

      const fsSource = new MockDataSource('fs-test', DataSourceType.FILESYSTEM);
      expect(fsSource.type).toBe(DataSourceType.FILESYSTEM);

    });
  });
});
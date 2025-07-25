import { getPaths } from '@/lib/cars';
import path from 'path';

describe('cars utilities', () => {
  describe('getPaths', () => {
    it('should return correct paths for given epoch and default data directory', () => {
      const paths = getPaths('123');
      
      expect(paths.base).toBe(path.join('/data', 'epoch-123'));
      expect(paths.car).toBe(path.join('/data', 'epoch-123', 'epoch-123.car'));
      expect(paths.indexes).toBe(path.join('/data', 'epoch-123', 'indexes'));
    });

    it('should return correct paths for given epoch and custom data directory', () => {
      const paths = getPaths('456', '/custom/data');
      
      expect(paths.base).toBe(path.join('/custom/data', 'epoch-456'));
      expect(paths.car).toBe(path.join('/custom/data', 'epoch-456', 'epoch-456.car'));
      expect(paths.indexes).toBe(path.join('/custom/data', 'epoch-456', 'indexes'));
    });

    it('should handle empty data directory', () => {
      const paths = getPaths('789', '');
      
      expect(paths.base).toBe(path.join('', 'epoch-789'));
      expect(paths.car).toBe(path.join('', 'epoch-789', 'epoch-789.car'));
      expect(paths.indexes).toBe(path.join('', 'epoch-789', 'indexes'));
    });

    it('should handle relative paths', () => {
      const paths = getPaths('100', './data');
      
      expect(paths.base).toBe(path.join('./data', 'epoch-100'));
      expect(paths.car).toBe(path.join('./data', 'epoch-100', 'epoch-100.car'));
      expect(paths.indexes).toBe(path.join('./data', 'epoch-100', 'indexes'));
    });

    it('should handle paths with trailing slashes', () => {
      const paths = getPaths('200', '/data/');
      
      expect(paths.base).toBe(path.join('/data/', 'epoch-200'));
      expect(paths.car).toBe(path.join('/data/', 'epoch-200', 'epoch-200.car'));
      expect(paths.indexes).toBe(path.join('/data/', 'epoch-200', 'indexes'));
    });
  });
});
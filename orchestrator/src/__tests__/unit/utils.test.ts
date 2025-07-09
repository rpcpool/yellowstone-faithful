import { cn, indexTypeToKebabCase, humanizeSize } from '@/lib/utils';
import { IndexType } from '@/lib/epochs';

describe('utils', () => {
  describe('cn', () => {
    it('should merge class names correctly', () => {
      expect(cn('text-red-500', 'bg-blue-500')).toBe('text-red-500 bg-blue-500');
    });

    it('should handle conditional classes', () => {
      expect(cn('base', { 'active': true, 'inactive': false })).toBe('base active');
    });

    it('should merge tailwind classes properly', () => {
      expect(cn('px-2 py-1', 'px-4')).toBe('py-1 px-4');
    });

    it('should handle undefined and null values', () => {
      expect(cn('base', undefined, null, 'end')).toBe('base end');
    });

    it('should handle arrays', () => {
      expect(cn(['text-sm', 'font-bold'])).toBe('text-sm font-bold');
    });
  });

  describe('indexTypeToKebabCase', () => {
    it('should convert CidToOffsetAndSize to kebab case', () => {
      expect(indexTypeToKebabCase('CidToOffsetAndSize' as IndexType)).toBe('cid-to-offset-and-size');
    });

    it('should convert SlotToTxIdx to kebab case', () => {
      expect(indexTypeToKebabCase('SlotToTxIdx' as IndexType)).toBe('slot-to-tx-idx');
    });

    it('should convert SigToTxIdx to kebab case', () => {
      expect(indexTypeToKebabCase('SigToTxIdx' as IndexType)).toBe('sig-to-tx-idx');
    });

    it('should convert SigExists to kebab case', () => {
      expect(indexTypeToKebabCase('SigExists' as IndexType)).toBe('sig-exists');
    });

    it('should handle single word index types', () => {
      expect(indexTypeToKebabCase('Single' as IndexType)).toBe('single');
    });

    it('should handle already lowercase strings', () => {
      expect(indexTypeToKebabCase('lowercase' as IndexType)).toBe('lowercase');
    });
  });

  describe('humanizeSize', () => {
    it('should format bytes correctly', () => {
      expect(humanizeSize(0)).toBe('0 B');
      expect(humanizeSize(1)).toBe('1 B');
      expect(humanizeSize(1023)).toBe('1023 B');
    });

    it('should format kilobytes correctly', () => {
      expect(humanizeSize(1024)).toBe('1 KB');
      expect(humanizeSize(1536)).toBe('1.5 KB');
      expect(humanizeSize(2048)).toBe('2 KB');
    });

    it('should format megabytes correctly', () => {
      expect(humanizeSize(1048576)).toBe('1 MB');
      expect(humanizeSize(1572864)).toBe('1.5 MB');
      expect(humanizeSize(5242880)).toBe('5 MB');
    });

    it('should format gigabytes correctly', () => {
      expect(humanizeSize(1073741824)).toBe('1 GB');
      expect(humanizeSize(2147483648)).toBe('2 GB');
      expect(humanizeSize(5368709120)).toBe('5 GB');
    });

    it('should format terabytes correctly', () => {
      expect(humanizeSize(1099511627776)).toBe('1 TB');
      expect(humanizeSize(2199023255552)).toBe('2 TB');
    });

    it('should handle string input', () => {
      expect(humanizeSize('1024')).toBe('1 KB');
      expect(humanizeSize('1048576')).toBe('1 MB');
    });

    it('should handle bigint input', () => {
      expect(humanizeSize(1024n)).toBe('1 KB');
      expect(humanizeSize(1048576n)).toBe('1 MB');
      expect(humanizeSize(1073741824n)).toBe('1 GB');
    });

    it('should round to 2 decimal places', () => {
      expect(humanizeSize(1536)).toBe('1.5 KB');
      expect(humanizeSize(1234567)).toBe('1.18 MB');
      expect(humanizeSize(1234567890)).toBe('1.15 GB');
    });
  });
});
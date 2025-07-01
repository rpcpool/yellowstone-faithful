import { DataSourceType } from '@/generated/prisma';
import { Source } from '@/lib/domain/source/entities/source';

/**
 * Determines the appropriate queue for a job based on the source configuration
 * @param source The source entity
 * @returns The queue name to use for scheduling jobs
 */
export function getQueueForSource(source: Source): string {
  // Each source should have its own dedicated queue
  // This allows each source's worker to process only its own jobs
  // Use dot separator as Faktory doesn't allow colons in queue names
  return `source.${source.id}`;
}

/**
 * Determines if a source requires local execution
 * @param source The source entity
 * @returns true if the source requires local execution
 */
export function isLocalSource(source: Source): boolean {
  return source.type === DataSourceType.FILESYSTEM;
}
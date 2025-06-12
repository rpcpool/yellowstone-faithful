import { runOnAllSources } from './run-on-all-sources';
import { IndexExistsResult, IndexType } from './types';

// Check all data sources for specific index type existence
export async function checkIndexExists(epochId: number, indexType: IndexType): Promise<IndexExistsResult[]> {
  const results = await runOnAllSources(
    'checkIndexExists',
    epochId,
    (source) => source.epochIndexExists(epochId, indexType)
  );

  return results.map(({ source, result }) => ({
    source,
    exists: result ?? false
  }));
} 
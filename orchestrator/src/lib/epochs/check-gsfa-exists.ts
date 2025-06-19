import { runOnAllSources } from './run-on-all-sources.js';
import { IndexExistsResult } from './types.js';

// Check all data sources for specific index type existence
export async function checkGsfaExists(epochId: number): Promise<IndexExistsResult[]> {
  const results = await runOnAllSources(
    'checkGsfaExists',
    epochId,
    (source) => source.epochGsfaIndexExists(epochId)
  );

  return results.map(({ source, result }) => ({
    source,
    exists: result ?? false
  }));
} 
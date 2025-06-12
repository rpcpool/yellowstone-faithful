import { runOnAllSources } from "./run-on-all-sources";
import { EpochExistsResult } from './types';

// Check all data sources for epoch existence
export async function checkEpochExists(epochId: number): Promise<EpochExistsResult[]> {
  const results = await runOnAllSources(
    'checkEpochExists',
    epochId,
    (source) => source.epochExists(epochId)
  );

  return results.map(({ source, result }) => ({
    source,
    exists: result ?? false
  }));
} 
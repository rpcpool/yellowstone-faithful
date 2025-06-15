import { checkSource } from './check-source';
import { getDataSourcesFromDB } from './data-sources-db';
import { EpochStatus } from './types';
import { updateEpochStatus } from './update-epoch-status';

// Check all index types across all data sources for an epoch
export async function checkAllSources(epochId: number): Promise<EpochStatus> {
  console.log(`[checkAllSources] Starting comprehensive check for epoch ${epochId}`);

  // Get data sources from database
  const dataSources = await getDataSourcesFromDB();
  
  // Check each data source - this will update the database for each source
  const sourceResults = await Promise.all(
    dataSources.map(async (source) => {
      console.log(`[checkAllSources] Checking source: ${source.name}`);
      return await checkSource(epochId, source);
    })
  );

  console.log(`[checkAllSources] Source results:`, sourceResults.map(r => 
    `${r.source}: ${r.indexesFound} indexes, GSFA: ${r.gsfaFound}`
  ).join(', '));

  // Now that all sources have been checked and the database updated,
  // determine the overall status based on what's in the database
  const finalStatus = await updateEpochStatus(epochId);
  console.log(`[checkAllSources] Final status for epoch ${epochId}: ${finalStatus}`);
  return finalStatus;
} 
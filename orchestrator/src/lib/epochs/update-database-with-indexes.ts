import { prisma } from '@/lib/infrastructure/persistence/prisma';
import { dataSources } from './data-sources';
import { IndexResult } from './types';

// Update database with found indexes
export async function updateDatabaseWithIndexes(
  epochId: number,
  epochStr: string,
  indexResults: IndexResult[],
): Promise<void> {
  console.log(`[updateDatabaseWithIndexes] Updating database for epoch string: ${epochStr}`);
  
  for (const indexResult of indexResults) {
    if (indexResult.availableIn.length > 0) {
      // Use the first available source for the location
      const firstSource = dataSources.find(ds => ds.name === indexResult.availableIn[0]);
      if (firstSource) {
        const location = await firstSource.getEpochIndexUrl(epochId, indexResult.type);
        
        // Fetch size information from the data source
        let size = 0;
        try {
          const stats = await firstSource.epochIndexStats(epochId, indexResult.type);
          size = stats.size;
          console.log(`[updateDatabaseWithIndexes] Fetched size for index ${indexResult.type}: ${size} bytes`);
        } catch (error) {
          console.warn(`[updateDatabaseWithIndexes] Failed to fetch size for index ${indexResult.type}:`, error);
          // Continue with size 0 if we can't fetch the size
        }
        
        console.log(`[updateDatabaseWithIndexes] Upserting index ${indexResult.type} with location: ${location}, size: ${size}`);
        
        await prisma.epochIndex.upsert({
          where: { 
            epoch_type_source: { 
              epoch: epochStr, 
              type: indexResult.type,
              source: firstSource.name
            } 
          },
          update: {
            status: 'Indexed',
            location,
            size,
          },
          create: {
            epoch: epochStr,
            type: indexResult.type,
            size,
            status: 'Indexed',
            source: firstSource.name,
            location,
          },
        });
      }
    } else {
      console.log(`[updateDatabaseWithIndexes] Index ${indexResult.type} not found in any source, skipping database update`);
    }
  }
} 
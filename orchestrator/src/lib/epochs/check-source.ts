import { prisma } from '@/lib/infrastructure/persistence/prisma';
import { DataSource } from '../interfaces/data-source';
import { IndexType, SourceCheckResult } from './types';

// Check a single data source for all index types of an epoch
export async function checkSource(epochId: number, source: DataSource): Promise<SourceCheckResult> {
  console.log(`[checkSource] Starting check for epoch ${epochId} in source ${source.name}`);

  const indexTypes = Object.values(IndexType);
  
  // Get the epoch string for database operations
  const epochStr = epochId.toString();
  
  // Check all index types concurrently
  const indexCheckPromises = indexTypes.map(async (indexType) => {
    console.log(`[checkSource] Checking ${source.name} for index ${indexType} of epoch ${epochId}...`);
    try {
      const exists = await source.epochIndexExists(epochId, indexType);
      console.log(`[checkSource] Index ${indexType} in ${source.name}: ${exists ? 'found' : 'not found'}`);
      
      // Get size and location information
      let size = 0;
      let location = '';
      
      if (exists) {
        try {
          const stats = await source.epochIndexStats(epochId, indexType);
          size = stats.size;
        } catch (error) {
          console.warn(`[checkSource] Failed to fetch size for index ${indexType}:`, error);
        }
        
        location = await source.getEpochIndexUrl(epochId, indexType);
        
        console.log(`[checkSource] Upserting index ${indexType} with location: ${location}, size: ${size}`);
        
        // Update database with this specific epoch+index+source combination
        if (!source.id) {
          throw new Error(`Source ${source.name} does not have an ID`);
        }
        
        await prisma.epochIndex.upsert({
          where: { 
            epoch_type_sourceId: { 
              epoch: epochStr, 
              type: indexType,
              sourceId: source.id
            } 
          },
          update: {
            status: 'Indexed',
            location,
            size,
            updatedAt: new Date()
          },
          create: {
            epoch: epochStr,
            type: indexType,
            size,
            status: 'Indexed',
            sourceId: source.id,
            location,
          },
        });
        
        return true; // Index found
      }
    } catch (error) {
      console.error(`[checkSource] Error checking index ${indexType} in ${source.name}:`, error);
      return false;
    }
  });

  // Check GSFA index concurrently with other indexes
  const gsfaCheckPromise = (async () => {
    try {
      const gsfaFound = await source.epochGsfaIndexExists(epochId);
      console.log(`[checkSource] Gsfa index in ${source.name}: ${gsfaFound ? 'found' : 'not found'}`);
      
      if (gsfaFound) {
        // Update GSFA index in database
        const location = await source.getEpochGsfaUrl(epochId);
        
        await prisma.epochGsfa.upsert({
          where: { 
            id: epochId 
          },
          update: {
            exists: true,
            location,
            updatedAt: new Date()
          },
          create: {
            id: epochId,
            epoch: epochStr,
            exists: true,
            location,
          },
        });
      }
      
      return gsfaFound;
    } catch (error) {
      console.error(`[checkSource] Error checking GSFA index in ${source.name}:`, error);
      return false;
    }
  })();

  // Wait for all checks to complete
  const [indexResults, gsfaFound] = await Promise.all([
    Promise.all(indexCheckPromises),
    gsfaCheckPromise
  ]);

  // Count how many indexes were found
  const indexesFound = indexResults.filter(Boolean).length;

  return {
    source: source.name,
    indexesFound,
    gsfaFound,
    success: true
  };
} 
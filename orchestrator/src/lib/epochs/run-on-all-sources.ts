import { DataSource } from '@/lib/interfaces/data-source';
import { getDataSourcesFromDB } from './data-sources-db';
import { DataSourceResult } from './types';

// Generic function to check across all data sources
export async function runOnAllSources<T>(
  operation: string,
  epochId: number,
  checker: (source: DataSource) => Promise<T>
): Promise<DataSourceResult<T>[]> {
  console.log(`[${operation}] Checking epoch ${epochId} across all data sources...`);
  
  const dataSources = await getDataSourcesFromDB();
  const results = await Promise.allSettled(
    dataSources.map(async (source) => {
      console.log(`[${operation}] Checking ${source.name} for epoch ${epochId}...`);
      const result = await checker(source);
      console.log(`[${operation}] ${source.name} response:`, result);
      return {
        source: source.name,
        result
      };
    })
  );

  return results.map((result, index) => {
    if (result.status === 'fulfilled') {
      return result.value;
    } else {
      console.error(`[${operation}] Error checking epoch ${epochId} in ${dataSources[index].name}:`, result.reason);
      return { source: dataSources[index].name, result: null };
    }
  });
} 
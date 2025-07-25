import { DataSource } from '@/lib/interfaces/data-source';
import { PrismaSourceRepository } from '@/lib/infrastructure/repositories/prisma-source-repository';
import { SourceFactory } from '@/lib/infrastructure/data-sources/source-factory';

// Cache for data sources to avoid repeated DB queries
let cachedDataSources: DataSource[] | null = null;
let cacheTimestamp: number = 0;
const CACHE_TTL = 60000; // 1 minute cache

export async function getDataSourcesFromDB(): Promise<DataSource[]> {
  const now = Date.now();
  
  // Return cached data if still valid
  if (cachedDataSources && (now - cacheTimestamp) < CACHE_TTL) {
    return cachedDataSources;
  }

  const sourceRepository = new PrismaSourceRepository();
  const sources = await sourceRepository.findEnabled();
  
  cachedDataSources = sources.map(source => SourceFactory.createDataSource(source));
  cacheTimestamp = now;
  
  return cachedDataSources;
}

export async function getDataSource(name: string): Promise<DataSource> {
  const dataSources = await getDataSourcesFromDB();
  const dataSource = dataSources.find(ds => ds.name === name);
  
  if (!dataSource) {
    throw new Error(`Data source ${name} not found`);
  }
  
  return dataSource;
}

// Clear cache when sources are updated
export function clearDataSourceCache() {
  cachedDataSources = null;
  cacheTimestamp = 0;
}
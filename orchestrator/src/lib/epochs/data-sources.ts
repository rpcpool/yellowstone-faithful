import { DataSource } from '@/lib/interfaces/data-source';
import { filesystemDataSource } from '@/sources/filesystem';
import { httpDataSource } from '@/sources/http';
import { s3DataSource } from '@/sources/s3';

// Available data sources
export const dataSources: DataSource[] = [
  s3DataSource,
  httpDataSource,
  filesystemDataSource,
]; 

export function getDataSource(name: string): DataSource {
  const dataSource = dataSources.find(ds => ds.name === name);
  if (!dataSource) {
    throw new Error(`Data source ${name} not found`);
  }
  return dataSource;
}
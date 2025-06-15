import { IndexType } from "@/generated/prisma";

export enum DataSourceType {
  S3 = "s3",
  HTTP = "http",
  FILESYSTEM = "filesystem",
}

export interface DataSource {
  id?: string; // Source ID from database
  name: string;
  type: DataSourceType;

  epochExists(epoch: number): Promise<boolean>;
  epochIndexExists(epoch: number, indexType: IndexType): Promise<boolean>;
  epochGsfaIndexExists(epoch: number): Promise<boolean>;
  epochIndexStats(epoch: number, indexType: IndexType): Promise<{
    size: number;
  }>;
  getEpochCid(epoch: number): Promise<string>;
  getEpochCarUrl(epoch: number): Promise<string>;
  getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string>;
  getEpochGsfaUrl(epoch: number): Promise<string>;
  getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string>;
}
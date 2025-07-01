import { DataSource, DataSourceType } from "@/lib/interfaces/data-source";
import { S3Client, ListObjectsV2Command } from "@aws-sdk/client-s3";
import { IndexType } from "@/generated/prisma";
import { indexTypeToKebabCase } from "@/lib/utils";

export interface S3Source extends DataSource {
  // S3Source inherits all methods from DataSource
  // Add S3-specific methods here
  
  bucket: string;
  client: S3Client;
}

export function createS3Source(id: string, name: string, config: {
  bucket: string;
  region: string;
  endpoint?: string;
  accessKeyId?: string;
  secretAccessKey?: string;
}): DataSource {
  const client = new S3Client({
    region: config.region,
    endpoint: config.endpoint,
    credentials: config.accessKeyId && config.secretAccessKey ? {
      accessKeyId: config.accessKeyId,
      secretAccessKey: config.secretAccessKey,
    } : undefined,
    forcePathStyle: true,
  });

  return {
    id,
    name,
    type: DataSourceType.S3,

    async epochExists(epoch: number): Promise<boolean> {
      const prefix = `epoch-${epoch}/`;
      try {
        const listCmd = new ListObjectsV2Command({
          Bucket: config.bucket,
          Prefix: prefix,
          MaxKeys: 1,
        });
        const result = await client.send(listCmd);
        return (result.Contents?.length || 0) > 0;
      } catch {
        return false;
      }
    },

    async epochIndexExists(epoch: number, indexType: IndexType): Promise<boolean> {
      const formattedIndexType = indexTypeToKebabCase(indexType);
      const key = `epoch-${epoch}/indexes/epoch-${epoch}-mainnet-${formattedIndexType}.index`;
      try {
        const listCmd = new ListObjectsV2Command({
          Bucket: config.bucket,
          Prefix: key,
          MaxKeys: 1,
        });
        const result = await client.send(listCmd);
        return (result.Contents?.length || 0) > 0;
      } catch {
        return false;
      }
    },

    async epochIndexStats(epoch: number, indexType: IndexType): Promise<{ size: number }> {
      const formattedIndexType = indexTypeToKebabCase(indexType);
      const key = `epoch-${epoch}/indexes/epoch-${epoch}-mainnet-${formattedIndexType}.index`;
      try {
        const listCmd = new ListObjectsV2Command({
          Bucket: config.bucket,
          Prefix: key,
          MaxKeys: 1,
        });
        const result = await client.send(listCmd);
        const object = result.Contents?.[0];
        return {
          size: object?.Size || 0,
        };
      } catch {
        return { size: 0 };
      }
    },

    async getEpochCid(): Promise<string> {
      throw new Error("CID retrieval from S3 not implemented");
    },

    async getEpochCarUrl(epoch: number): Promise<string> {
      return `s3://${config.bucket}/epoch-${epoch}/epoch-${epoch}.car`;
    },

    async getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string> {
      const formattedIndexType = indexTypeToKebabCase(indexType);
      return `s3://${config.bucket}/epoch-${epoch}/indexes/epoch-${epoch}-mainnet-${formattedIndexType}.index`;
    },

    async epochGsfaIndexExists(epoch: number): Promise<boolean> {
      const key = `epoch-${epoch}/indexes/epoch-${epoch}-mainnet-gsfa.indexdir`;
      try {
        const listCmd = new ListObjectsV2Command({
          Bucket: config.bucket,
          Prefix: key,
          MaxKeys: 1,
        });
        const result = await client.send(listCmd);
        return (result.Contents?.length || 0) > 0;
      } catch {
        return false;
      }
    },

    async getEpochGsfaUrl(epoch: number): Promise<string> {
      return `s3://${config.bucket}/epoch-${epoch}/indexes/epoch-${epoch}-mainnet-gsfa.indexdir`;
    },

    async getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string> {
      return `s3://${config.bucket}/epoch-${epoch}/indexes/epoch-${epoch}-gsfa.indexdir.tar.zstd`;
    },
  };
} 
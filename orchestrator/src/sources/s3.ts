import { IndexType } from "@/generated/prisma";
import { ListObjectsV2Command, S3Client } from "@aws-sdk/client-s3";
import { S3Source } from "@/lib/data-sources/s3-source";
import { DataSourceType } from "../lib/interfaces/data-source";
import { indexTypeToKebabCase } from "../lib/utils";

export const s3DataSource: S3Source = {
  name: "S3",
  type: DataSourceType.S3,
  bucket: "solana-cars",
  client: new S3Client({
    region: 'us-east-1',
    endpoint: `https://${process.env.AWS_ENDPOINT}`,
    credentials: {
      accessKeyId: process.env.AWS_ACCESS_KEY_ID!,
      secretAccessKey: process.env.AWS_SECRET_ACCESS_KEY!,
    },
    forcePathStyle: true,
  }),

  async epochExists(epoch: number): Promise<boolean> {
    const prefix = `epoch-${epoch}/`;
    try {
      const listCmd = new ListObjectsV2Command({
        Bucket: this.bucket,
        Prefix: prefix,
        MaxKeys: 1,
      });
      const result = await this.client.send(listCmd);
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
        Bucket: this.bucket,
        Prefix: key,
        MaxKeys: 1,
      });
      const result = await this.client.send(listCmd);
      return (result.Contents?.length || 0) > 0;
    } catch {
      return false;
    }
  },

  async epochIndexStats(epoch: number, indexType: IndexType): Promise<{
    size: number;
  }> {
    const formattedIndexType = indexTypeToKebabCase(indexType);
    const key = `epoch-${epoch}/indexes/epoch-${epoch}-mainnet-${formattedIndexType}.index`;
    try {
      const listCmd = new ListObjectsV2Command({
        Bucket: this.bucket,
        Prefix: key,
        MaxKeys: 1,
      });
      const result = await this.client.send(listCmd);
      const object = result.Contents?.[0];
      return {
        size: object?.Size || 0,
      };
    } catch {
      return { size: 0 };
    }
  },

  async getEpochCid(): Promise<string> {
    // S3 doesn't store CID files directly, this would need to be implemented
    // based on your specific S3 structure
    throw new Error("CID retrieval from S3 not implemented");
  },

  async getEpochCarUrl(epoch: number): Promise<string> {
    return `s3://${this.bucket}/epoch-${epoch}/epoch-${epoch}.car`;
  },

  async getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string> {
    const formattedIndexType = indexTypeToKebabCase(indexType);
    return `s3://${this.bucket}/epoch-${epoch}/indexes/epoch-${epoch}-mainnet-${formattedIndexType}.index`;
  },

  async epochGsfaIndexExists(epoch: number): Promise<boolean> {
    const key = `epoch-${epoch}/indexes/epoch-${epoch}-mainnet-gsfa.indexdir`;
    try {
      const listCmd = new ListObjectsV2Command({
        Bucket: this.bucket,
        Prefix: key,
        MaxKeys: 1,
      });
      const result = await this.client.send(listCmd);
      return (result.Contents?.length || 0) > 0;
    } catch {
      return false;
    }
  },

  async getEpochGsfaUrl(epoch: number): Promise<string> {
    return `s3://${this.bucket}/epoch-${epoch}/indexes/epoch-${epoch}-mainnet-gsfa.indexdir`;
  },

  async getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string> {
    return `s3://${this.bucket}/epoch-${epoch}/indexes/epoch-${epoch}-gsfa.indexdir.tar.zstd`;
  },
}; 
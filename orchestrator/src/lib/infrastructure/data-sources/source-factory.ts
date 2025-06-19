import { DataSource, DataSourceType as IDataSourceType } from '@/lib/interfaces/data-source';
import { 
  Source, 
  S3Configuration, 
  HTTPConfiguration, 
  FilesystemConfiguration
} from '@/lib/domain/source/entities/source';
import { S3Client } from '@aws-sdk/client-s3';
import { DataSourceType } from '@/generated/prisma';
import { indexTypeToKebabCase } from '@/lib/utils';
import { IndexType } from '@/generated/prisma';
import { ListObjectsV2Command } from '@aws-sdk/client-s3';

export class SourceFactory {
  static createDataSource(source: Source): DataSource {
    const config = source.getTypedConfiguration();

    switch (source.type) {
      case DataSourceType.S3:
        return this.createS3DataSource(source.id, source.name, config as S3Configuration);
      
      case DataSourceType.HTTP:
        return this.createHTTPDataSource(source.id, source.name, config as HTTPConfiguration);
      
      case DataSourceType.FILESYSTEM:
        return this.createFilesystemDataSource(source.id, source.name, config as FilesystemConfiguration);
      
      default:
        throw new Error(`Unknown source type: ${source.type}`);
    }
  }

  private static createS3DataSource(id: string, name: string, config: {
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
      type: IDataSourceType.S3,

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

  private static createHTTPDataSource(id: string, name: string, config: {
    host: string;
    path: string;
    username?: string;
    password?: string;
  }): DataSource {
    const basicAuth = config.username && config.password ? {
      username: config.username,
      password: config.password,
    } : undefined;

    return {
      id,
      name,
      type: IDataSourceType.HTTP,

      async epochExists(epoch: number): Promise<boolean> {
        const headers: Record<string, string> = {};
        if (basicAuth) {
          headers["Authorization"] = `Basic ${Buffer.from(`${basicAuth.username}:${basicAuth.password}`).toString("base64")}`;
        }

        const response = await fetch(`${config.host}${config.path}/${epoch}/`, {
          headers,
        });
        
        if (response.status === 401) {
          throw new Error("Unauthorized: Invalid credentials or authentication required");
        }
        
        return response.ok;
      },

      async epochIndexExists(epoch: number, indexType: IndexType): Promise<boolean> {
        const cid = await this.getEpochCid(epoch);
        const formattedIndexType = indexTypeToKebabCase(indexType);
        const url = `${config.host}${config.path}/epoch-${epoch}/indexes/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`;

        const headers: Record<string, string> = {};
        if (basicAuth) {
          headers["Authorization"] = `Basic ${Buffer.from(`${basicAuth.username}:${basicAuth.password}`).toString("base64")}`;
        }

        const response = await fetch(url, {
          method: "HEAD",
          redirect: "follow",
          headers,
        });
        
        if (response.status === 401) {
          throw new Error("Unauthorized: Invalid credentials or authentication required");
        }
        
        return response.status === 200;
      },

      async epochGsfaIndexExists(epoch: number): Promise<boolean> {
        const cid = await this.getEpochCid(epoch);
        const url = `${config.host}${config.path}/epoch-${epoch}/indexes/epoch-${epoch}-${cid}-mainnet-gsfa.indexdir`;

        const headers: Record<string, string> = {};
        if (basicAuth) {
          headers["Authorization"] = `Basic ${Buffer.from(`${basicAuth.username}:${basicAuth.password}`).toString("base64")}`;
        }

        const response = await fetch(url, {
          method: "HEAD",
          redirect: "follow",
          headers,
        });
        
        if (response.status === 401) {
          throw new Error("Unauthorized: Invalid credentials or authentication required");
        }
        
        return response.status === 200;
      },

      async epochIndexStats(epoch: number, indexType: IndexType): Promise<{ size: number }> {
        const headers: Record<string, string> = {};
        if (basicAuth) {
          headers["Authorization"] = `Basic ${Buffer.from(`${basicAuth.username}:${basicAuth.password}`).toString("base64")}`;
        }

        const cid = await this.getEpochCid(epoch);
        const formattedIndexType = indexTypeToKebabCase(indexType);
        const response = await fetch(`${config.host}${config.path}/epoch-${epoch}/indexes/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`, {
          headers,
        });
        
        if (response.status === 401) {
          throw new Error("Unauthorized: Invalid credentials or authentication required");
        }

        return {
          size: parseInt(response.headers.get("Content-Length") || "0", 10),
        };
      },

      async getEpochCid(epoch: number): Promise<string> {
        const headers: Record<string, string> = {};
        if (basicAuth) {
          headers["Authorization"] = `Basic ${Buffer.from(`${basicAuth.username}:${basicAuth.password}`).toString("base64")}`;
        }

        const response = await fetch(`${config.host}${config.path}/epoch-${epoch}/epoch-${epoch}.cid`, {
          headers,
        });
        
        if (response.status === 401) {
          throw new Error("Unauthorized: Invalid credentials or authentication required");
        }

        return (await response.text()).trim();
      },

      async getEpochCarUrl(epoch: number): Promise<string> {
        return `${config.host}${config.path}/epoch-${epoch}/epoch-${epoch}.car`;
      },

      async getEpochGsfaUrl(epoch: number): Promise<string> {
        const cid = await this.getEpochCid(epoch);
        return `${config.path}/epoch-${epoch}/indexes/epoch-${epoch}-${cid}-mainnet-gsfa.indexdir`;
      },

      async getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string> {
        const cid = await this.getEpochCid(epoch);
        const formattedIndexType = indexTypeToKebabCase(indexType);
        return `${config.host}${config.path}/epoch-${epoch}/indexes/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`;
      },

      async getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string> {
        return `${config.host}${config.path}/epoch-${epoch}/indexes/epoch-${epoch}-gsfa.indexdir.tar.zstd`;
      },
    };
  }

  private static createFilesystemDataSource(id: string, name: string, config: {
    basePath: string;
  }): DataSource {
    // Simplified implementation - would need actual filesystem checks
    /* eslint-disable @typescript-eslint/no-unused-vars */
    return {
      id,
      name,
      type: IDataSourceType.FILESYSTEM,

      async epochExists(_epoch: number): Promise<boolean> {
        // Would check if directory exists at ${basePath}/epoch-${epoch}
        return false;
      },

      async epochIndexExists(epoch: number, indexType: IndexType): Promise<boolean> {
        return false;
      },

      async epochGsfaIndexExists(epoch: number): Promise<boolean> {
        return false;
      },

      async epochIndexStats(epoch: number, indexType: IndexType): Promise<{ size: number }> {
        return { size: 0 };
      },

      async getEpochCid(epoch: number): Promise<string> {
        throw new Error("Not implemented");
      },

      async getEpochCarUrl(epoch: number): Promise<string> {
        return `file://${config.basePath}/epoch-${epoch}/epoch-${epoch}.car`;
      },

      async getEpochGsfaUrl(epoch: number): Promise<string> {
        return `file://${config.basePath}/epoch-${epoch}/indexes/epoch-${epoch}-mainnet-gsfa.indexdir`;
      },

      async getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string> {
        const formattedIndexType = indexTypeToKebabCase(indexType);
        return `file://${config.basePath}/epoch-${epoch}/indexes/epoch-${epoch}-mainnet-${formattedIndexType}.index`;
      },

      async getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string> {
        return `file://${config.basePath}/epoch-${epoch}/indexes/epoch-${epoch}-gsfa.indexdir.tar.zstd`;
      },
    };
    /* eslint-enable @typescript-eslint/no-unused-vars */
  }

}

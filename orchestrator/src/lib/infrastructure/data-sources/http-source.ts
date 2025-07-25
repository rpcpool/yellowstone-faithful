import { DataSource, DataSourceType } from "@/lib/interfaces/data-source";
import { IndexType } from "@/generated/prisma";
import { indexTypeToKebabCase } from "@/lib/utils";

export interface HTTPSource extends DataSource {
  host: string;
  path: string;
  basicAuth?: {
    username: string;
    password: string;
  };
}

export function createHTTPSource(id: string, name: string, config: {
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
    type: DataSourceType.HTTP,

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
import { IndexType } from "@/generated/prisma";
import { HTTPSource } from "@/lib/infrastructure/data-sources/http-source";
import { DataSourceType } from "@/lib/interfaces/data-source";
import { indexTypeToKebabCase } from "@/lib/utils";

export const httpDataSource: HTTPSource = {
  name: "HTTP",
  type: DataSourceType.HTTP,

  host: process.env.INDEX_HOST || "http://localhost:8080",
  path: "/cars",

  basicAuth: process.env.INDEX_USER && process.env.INDEX_PASS ? {
    username: process.env.INDEX_USER,
    password: process.env.INDEX_PASS,
  } : undefined,

  async epochExists(epoch: number): Promise<boolean> {
    const headers: Record<string, string> = {};
    if (this.basicAuth) {
      headers["Authorization"] = `Basic ${Buffer.from(`${this.basicAuth.username}:${this.basicAuth.password}`).toString("base64")}`;
    }

    const response = await fetch(`${this.host}${this.path}/${epoch}/`, {
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
    
    const url = `${this.host}${this.path}/epoch-${epoch}/indexes/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`;

    const headers: Record<string, string> = {};
    if (this.basicAuth) {
      headers["Authorization"] = `Basic ${Buffer.from(`${this.basicAuth.username}:${this.basicAuth.password}`).toString("base64")}`;
    }

    console.log(`Checking if ${url} exists`);
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
    
    const url = `${this.host}${this.path}/epoch-${epoch}/indexes/epoch-${epoch}-${cid}-mainnet-gsfa.indexdir`;

    const headers: Record<string, string> = {};
    if (this.basicAuth) {
      headers["Authorization"] = `Basic ${Buffer.from(`${this.basicAuth.username}:${this.basicAuth.password}`).toString("base64")}`;
    }

    console.log(`Checking if ${url} exists`);
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

  async epochIndexStats(epoch: number, indexType: IndexType): Promise<{
    size: number;
  }> {
    const headers: Record<string, string> = {};
    if (this.basicAuth) {
      headers["Authorization"] = `Basic ${Buffer.from(`${this.basicAuth.username}:${this.basicAuth.password}`).toString("base64")}`;
    }

    const cid = await this.getEpochCid(epoch);

    const formattedIndexType = indexTypeToKebabCase(indexType);
    const response = await fetch(`${this.host}${this.path}/epoch-${epoch}/indexes/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`, {
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

    if (this.basicAuth) {
      headers["Authorization"] = `Basic ${Buffer.from(`${this.basicAuth.username}:${this.basicAuth.password}`).toString("base64")}`;
    }

    const response = await fetch(`${this.host}${this.path}/epoch-${epoch}/epoch-${epoch}.cid`, {
      headers,
    });
    
    if (response.status === 401) {
      throw new Error("Unauthorized: Invalid credentials or authentication required");
    }

    return (await response.text()).trim();
  },

  async getEpochCarUrl(epoch: number): Promise<string> {
    return `${this.host}${this.path}/epoch-${epoch}/epoch-${epoch}.car`;
  },

  async getEpochGsfaUrl(epoch: number): Promise<string> {
    const cid = await this.getEpochCid(epoch);
    return `/data/cars/epoch-${epoch}/indexes/epoch-${epoch}-${cid}-mainnet-gsfa.indexdir`;
  },

  async getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string> {
    const cid = await this.getEpochCid(epoch);
    const formattedIndexType = indexTypeToKebabCase(indexType);
    return `${this.host}${this.path}/epoch-${epoch}/indexes/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`;
  },

  async getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string> {
    return `${this.host}${this.path}/epoch-${epoch}/indexes/epoch-${epoch}-gsfa.indexdir.tar.zstd`;
  },
};
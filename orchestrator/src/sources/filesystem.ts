import { IndexType } from "@/generated/prisma";
import { FileSystemSource } from "@/lib/data-sources/filesystem-source";
import { DataSourceType } from "@/lib/interfaces/data-source";
import { indexTypeToKebabCase } from "@/lib/utils";
import { existsSync, statSync } from "fs";

export const filesystemDataSource: FileSystemSource = {
  name: "Filesystem",
  type: DataSourceType.FILESYSTEM,

  getBasePath: () => "/data/indexes",

  async fileExists(filePath: string): Promise<boolean> {
    return existsSync(filePath);
  },

  getEpochFilePath(epoch: number): string {
    return `${this.getBasePath()}/${epoch}/`;
  },

  async epochExists(epoch: number): Promise<boolean> {
    return existsSync(`${this.getBasePath()}/${epoch}/`);
  },

  async epochIndexExists(epoch: number, indexType: IndexType): Promise<boolean> {
    try {
      const cid = await this.getEpochCid(epoch);
      const formattedIndexType = indexTypeToKebabCase(indexType);
      const filePath = `${this.getBasePath()}/${epoch}/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`;

      console.log(`[localDataSource] Checking if ${filePath} exists`);

      return existsSync(filePath);
    } catch {
      // If we can't get the CID or any other error occurs, the index doesn't exist
      return false;
    }
  },

  async epochGsfaIndexExists(epoch: number): Promise<boolean> {
    try {
      const cid = await this.getEpochCid(epoch);
      const filePath = `${this.getBasePath()}/${epoch}/epoch-${epoch}-${cid}-mainnet-gsfa.indexdir`;
      return existsSync(filePath);
    } catch {
      // If we can't get the CID or any other error occurs, the GSFA index doesn't exist
      return false;
    }
  },

  async epochIndexStats(epoch: number, indexType: IndexType): Promise<{
    size: number;
  }> {
    try {
      const cid = await this.getEpochCid(epoch);
      const formattedIndexType = indexTypeToKebabCase(indexType);
      const filePath = `${this.getBasePath()}/${epoch}/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`;

      if (!existsSync(filePath)) {
        return { size: 0 };
      }

      const stats = statSync(filePath);
      return { size: stats.size };
    } catch (error) {
      console.error(`[localDataSource] Error getting file stats for epoch ${epoch}, index ${indexType}:`, error);
      return { size: 0 };
    }
  },

  async getEpochCid(epoch: number): Promise<string> {
    const response = await fetch(`https://files.old-faithful.net/${epoch}/epoch-${epoch}.cid`);
    return (await response.text()).trim();
  },

  async getEpochCarUrl(epoch: number): Promise<string> {
    return `${this.getBasePath()}/${epoch}/epoch-${epoch}.car`;
  },

  async getEpochGsfaUrl(epoch: number): Promise<string> {
    try {
      const cid = await this.getEpochCid(epoch);
      return `${this.getBasePath()}/${epoch}/epoch-${epoch}-${cid}-mainnet-gsfa.indexdir`;
    } catch {
      // If CID file doesn't exist, return a fallback URL
      return `${this.getBasePath()}/${epoch}/epoch-${epoch}-unknown-mainnet-gsfa.indexdir`;
    }
  },

  async getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string> {
    return `https://files.old-faithful.net/${epoch}/epoch-${epoch}-gsfa.index.tar.zstd`;
  },

  async getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string> {
    try {
      const cid = await this.getEpochCid(epoch);
      const formattedIndexType = indexTypeToKebabCase(indexType);
      console.log(`[getEpochIndexUrl] ${this.getBasePath()}/${epoch}/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`);
      return `${this.getBasePath()}/${epoch}/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`;
    } catch {
      // If CID file doesn't exist, return a fallback URL
      const formattedIndexType = indexTypeToKebabCase(indexType);
      return `${this.getBasePath()}/${epoch}/epoch-${epoch}-unknown-mainnet-${formattedIndexType}.index`;
    }
  },
};
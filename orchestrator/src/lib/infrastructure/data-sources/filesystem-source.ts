import { IndexType } from "@/generated/prisma";
import { DataSource, DataSourceType } from "@/lib/interfaces/data-source";
import { indexTypeToKebabCase } from "@/lib/utils";
import { promises as fs } from "fs";

export interface FileSystemSource extends DataSource {
  // FileSystemSource inherits all methods from DataSource
  // Add any filesystem-specific methods here
  
  /**
   * Get the base directory path for this filesystem source
   */
  getBasePath(): string;
  
  /**
   * Check if a file exists at the given path
   */
  fileExists(filePath: string): Promise<boolean>;
  
  /**
   * Get the full file path for an epoch
   */
  getEpochFilePath(epoch: number): string;
}

export function createFilesystemSource(id: string, name: string, config: {
  basePath: string;
}): DataSource {
  // Simplified implementation - would need actual filesystem checks
  /* eslint-disable @typescript-eslint/no-unused-vars */
  return {
    id,
    name,
    type: DataSourceType.FILESYSTEM,

    async epochExists(_epoch: number): Promise<boolean> {
      // Check if the CAR file exists
      const carUrl = await this.getEpochCarUrl(_epoch);
      try {
        await fs.access(carUrl);
        return true;
      } catch {
        return false;
      }
    },

    async epochIndexExists(_epoch: number, _indexType: IndexType): Promise<boolean> {
      const filePath = await this.getEpochIndexUrl(_epoch, _indexType);
      try {
        await fs.access(filePath);
        return true;
      } catch {
        return false;
      }
    },

    async epochGsfaIndexExists(_epoch: number): Promise<boolean> {
      const filePath = await this.getEpochGsfaUrl(_epoch);
      try {
        await fs.access(filePath);
        return true;
      } catch {
        return false;
      }
    },

    async epochIndexStats(epoch: number, indexType: IndexType): Promise<{ size: number }> {
      const filePath = await this.getEpochIndexUrl(epoch, indexType);
      try {
        const stat = await fs.stat(filePath);
        return { size: stat.size };
      } catch {
        return { size: 0 };
      }
    },

    async getEpochCid(epoch: number): Promise<string> {
      const response = await fetch(`https://files.old-faithful.net/${epoch}/epoch-${epoch}.cid`);
      return (await response.text()).trim();
    },

    async getEpochCarUrl(epoch: number): Promise<string> {
      return `${config.basePath}/${epoch}/epoch-${epoch}.car`;
    },

    async getEpochGsfaUrl(epoch: number): Promise<string> {
      const cid = await this.getEpochCid(epoch);
      return `${config.basePath}/${epoch}/epoch-${epoch}-${cid}-mainnet-gsfa.indexdir`;
    },

    async getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string> {
      const cid = await this.getEpochCid(epoch);
      const formattedIndexType = indexTypeToKebabCase(indexType);
      return `${config.basePath}/${epoch}/epoch-${epoch}-${cid}-mainnet-${formattedIndexType}.index`;
    },

    async getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string> {
      const cid = await this.getEpochCid(epoch);
      return `${config.basePath}/${epoch}/epoch-${epoch}-${cid}-mainnet-gsfa.indexdir.tar.zstd`;
    },
  };
  /* eslint-enable @typescript-eslint/no-unused-vars */
} 
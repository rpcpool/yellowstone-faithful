import { DataSource, DataSourceType } from "@/lib/interfaces/data-source";
import { IndexType } from "@/generated/prisma";
import { indexTypeToKebabCase } from "@/lib/utils";

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
      return `file://${config.basePath}/${epoch}/epoch-${epoch}.car`;
    },

    async getEpochGsfaUrl(epoch: number): Promise<string> {
      return `file://${config.basePath}/${epoch}/indexes/epoch-${epoch}-mainnet-gsfa.indexdir`;
    },

    async getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string> {
      const formattedIndexType = indexTypeToKebabCase(indexType);
      return `file://${config.basePath}/${epoch}/indexes/epoch-${epoch}-mainnet-${formattedIndexType}.index`;
    },

    async getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string> {
      return `file://${config.basePath}/${epoch}/indexes/epoch-${epoch}-gsfa.indexdir.tar.zstd`;
    },
  };
  /* eslint-enable @typescript-eslint/no-unused-vars */
} 
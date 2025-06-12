import { DataSource } from "@/lib/interfaces/data-source";

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
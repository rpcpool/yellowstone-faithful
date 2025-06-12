import { IndexType } from "@/generated/prisma";
import { DataSource, DataSourceType } from "../lib/interfaces/data-source";
import { indexTypeToKebabCase } from "../lib/utils";

export const ofDataSource: DataSource = {
  name: "Old Faithful",
  type: DataSourceType.OF,

  async epochExists(epoch: number): Promise<boolean> {
    const response = await fetch(`https://files.old-faithful.net/${epoch}/epoch-${epoch}.cid`, {
      method: "HEAD",
      redirect: "follow",
    });
    return response.status === 200;
  },

  async epochIndexExists(epoch: number, indexType: IndexType): Promise<boolean> {
    try {
      // First get the CID for this epoch
      const cidResponse = await fetch(`https://files.old-faithful.net/${epoch}/epoch-${epoch}.cid`);
      if (!cidResponse.ok) {
        console.log(`[ofDataSource] CID not found for epoch ${epoch}`);
        return false;
      }
      
      const cid = (await cidResponse.text()).trim();
      const indexTypeKebab = indexTypeToKebabCase(indexType);
      
      // Check if the index file exists with the correct pattern
      const indexUrl = `https://files.old-faithful.net/${epoch}/epoch-${epoch}-${cid}-mainnet-${indexTypeKebab}.index`;
      console.log(`[ofDataSource] Checking index URL: ${indexUrl}`);
      
      const response = await fetch(indexUrl, {
        method: "HEAD",
        redirect: "follow",
      });
      
      const exists = response.status === 200;
      console.log(`[ofDataSource] Index ${indexType} for epoch ${epoch} exists: ${exists}`);
      return exists;
    } catch (error) {
      console.error(`[ofDataSource] Error checking index ${indexType} for epoch ${epoch}:`, error);
      return false;
    }
  },

  async epochIndexStats(epoch: number, indexType: IndexType): Promise<{
    size: number;
  }> {
    try {
      // Get the CID for this epoch
      const cidResponse = await fetch(`https://files.old-faithful.net/${epoch}/epoch-${epoch}.cid`);
      if (!cidResponse.ok) {
        console.log(`[ofDataSource] CID not found for epoch ${epoch} when fetching stats`);
        return { size: 0 };
      }
      
      const cid = (await cidResponse.text()).trim();
      const indexTypeKebab = indexTypeToKebabCase(indexType);
      
      // Fetch the index file with the correct pattern
      const indexUrl = `https://files.old-faithful.net/${epoch}/epoch-${epoch}-${cid}-mainnet-${indexTypeKebab}.index`;
      console.log(`[ofDataSource] Fetching stats from URL: ${indexUrl}`);
      
      const response = await fetch(indexUrl, {
        method: "HEAD",
        redirect: "follow",
      });
      
      if (!response.ok) {
        console.log(`[ofDataSource] Failed to fetch stats for epoch ${epoch}, index ${indexType}: ${response.status}`);
        return { size: 0 };
      }
      
      const contentLength = response.headers.get("Content-Length");
      const size = parseInt(contentLength || "0", 10);
      console.log(`[ofDataSource] Content-Length header: "${contentLength}"`);
      console.log(`[ofDataSource] Parsed size: ${size}`);
      console.log(`[ofDataSource] Successfully fetched size for epoch ${epoch}, index ${indexType}: ${size} bytes`);
      
      return { size };
    } catch (error) {
      console.error(`[ofDataSource] Error fetching stats for epoch ${epoch}, index ${indexType}:`, error);
      return { size: 0 };
    }
  },

  async getEpochCid(epoch: number): Promise<string> {
    const response = await fetch(`https://files.old-faithful.net/${epoch}/epoch-${epoch}.cid`);
    return (await response.text()).trim();
  },

  async getEpochCarUrl(epoch: number): Promise<string> {
    return `https://files.old-faithful.net/${epoch}/epoch-${epoch}.car`;
  },

  async getEpochIndexUrl(epoch: number, indexType: IndexType): Promise<string> {
    try {
      const cid = await this.getEpochCid(epoch);
      const indexTypeKebab = indexTypeToKebabCase(indexType);
      return `https://files.old-faithful.net/${epoch}/epoch-${epoch}-${cid}-mainnet-${indexTypeKebab}.index`;
    } catch (error) {
      console.error(`[ofDataSource] Error getting index URL for epoch ${epoch}, type ${indexType}:`, error);
      // Fallback to old pattern if CID fetch fails
      const indexTypeKebab = indexTypeToKebabCase(indexType);
      return `https://files.old-faithful.net/${epoch}/epoch-${epoch}-mainnet-${indexTypeKebab}.index`;
    }
  },

  async epochGsfaIndexExists(epoch: number): Promise<boolean> {
    try {
      // First get the CID for this epoch
      const cidResponse = await fetch(`https://files.old-faithful.net/${epoch}/epoch-${epoch}.cid`);
      if (!cidResponse.ok) {
        console.log(`[ofDataSource] CID not found for epoch ${epoch}`);
        return false;
      }
      
      const cid = (await cidResponse.text()).trim();
      
      // Check if the GSFA index directory exists with the correct pattern
      const gsfaUrl = `https://files.old-faithful.net/${epoch}/epoch-${epoch}-${cid}-mainnet-gsfa.indexdir`;
      console.log(`[ofDataSource] Checking GSFA URL: ${gsfaUrl}`);
      
      const response = await fetch(gsfaUrl, {
        method: "HEAD",
        redirect: "follow",
      });
      
      const exists = response.status === 200;
      console.log(`[ofDataSource] GSFA index for epoch ${epoch} exists: ${exists}`);
      return exists;
    } catch (error) {
      console.error(`[ofDataSource] Error checking GSFA index for epoch ${epoch}:`, error);
      return false;
    }
  },

  async getEpochGsfaUrl(epoch: number): Promise<string> {
    const cid = await this.getEpochCid(epoch);
    return `https://files.old-faithful.net/${epoch}/epoch-${epoch}-${cid}-mainnet-gsfa.indexdir`;
  },

  async getEpochGsfaIndexArchiveUrl(epoch: number): Promise<string> {
    return `https://files.old-faithful.net/${epoch}/epoch-${epoch}-gsfa.index.tar.zstd`;
  },
};
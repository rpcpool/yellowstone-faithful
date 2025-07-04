import { Epoch } from '../entities/epoch';
import { EpochId } from '../value-objects/epoch-id';
import { EpochRepository } from '../repositories/epoch-repository';
import { DataSource } from '@/lib/domain/data-source/interfaces/data-source';

export class EpochDomainService {
  constructor(
    private readonly epochRepository: EpochRepository
  ) {}

  /**
   * Check if an epoch exists across all provided data sources
   */
  async checkEpochExistsInSources(
    epochId: EpochId,
    sources: DataSource[]
  ): Promise<{ exists: boolean; sources: string[] }> {
    const existingSources: string[] = [];

    for (const source of sources) {
      const exists = await source.epochExists(epochId);
      if (exists) {
        existingSources.push(source.getName());
      }
    }

    return {
      exists: existingSources.length > 0,
      sources: existingSources
    };
  }

  /**
   * Refresh epoch status based on current indexes
   */
  async refreshEpochStatus(epochId: EpochId): Promise<void> {
    const epoch = await this.epochRepository.findById(epochId);
    if (!epoch) {
      throw new Error(`Epoch ${epochId.getValue()} not found`);
    }

    epoch.updateStatusBasedOnIndexes();
    await this.epochRepository.save(epoch);
  }

  /**
   * Get or create an epoch
   */
  async getOrCreateEpoch(epochId: EpochId): Promise<Epoch> {
    let epoch = await this.epochRepository.findById(epochId);
    
    if (!epoch) {
      epoch = Epoch.create(epochId);
      await this.epochRepository.save(epoch);
    }
    
    return epoch;
  }

  /**
   * Check if an epoch is complete (has all indexes and GSFA)
   */
  async isEpochComplete(epochId: EpochId): Promise<boolean> {
    const epoch = await this.epochRepository.findById(epochId);
    if (!epoch) {
      return false;
    }

    return epoch.hasAllStandardIndexes() && epoch.hasGsfaIndex();
  }

  /**
   * Get the latest epoch number
   */
  async getLatestEpochNumber(): Promise<number | null> {
    const latestEpoch = await this.epochRepository.getLatestEpoch();
    return latestEpoch ? latestEpoch.getId().getValue() : null;
  }
}
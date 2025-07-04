import { Source, SourceConfiguration } from '../entities/source';
import { SourceRepository } from '../repositories/source-repository';
import { DataSourceType } from '@/generated/prisma';

export class SourceDomainService {
  constructor(private readonly sourceRepository: SourceRepository) {}

  async createSource(
    name: string,
    type: DataSourceType,
    configuration: SourceConfiguration,
    enabled: boolean = true
  ): Promise<Source> {
    // Check if source with same name already exists
    const existingSource = await this.sourceRepository.findByName(name);
    if (existingSource) {
      throw new Error(`Source with name "${name}" already exists`);
    }

    // Validate configuration based on type
    this.validateConfiguration(type, configuration);

    // Create new source
    const source = Source.create(name, type, configuration, enabled);

    // Save to repository and get back the source with generated ID
    const savedSource = await this.sourceRepository.save(source);

    return savedSource;
  }

  async updateSource(
    id: string,
    updates: {
      name?: string;
      configuration?: SourceConfiguration;
      enabled?: boolean;
    }
  ): Promise<Source> {
    const source = await this.sourceRepository.findById(id);
    if (!source) {
      throw new Error(`Source with id "${id}" not found`);
    }

    // Update name if provided and check for duplicates
    if (updates.name && updates.name !== source.name) {
      const existingSource = await this.sourceRepository.findByName(updates.name);
      if (existingSource) {
        throw new Error(`Source with name "${updates.name}" already exists`);
      }
      source.updateName(updates.name);
    }

    // Update configuration if provided
    if (updates.configuration) {
      this.validateConfiguration(source.type, updates.configuration);
      source.updateConfiguration(updates.configuration);
    }

    // Update enabled status if provided
    if (updates.enabled !== undefined) {
      if (updates.enabled) {
        source.enable();
      } else {
        source.disable();
      }
    }

    await this.sourceRepository.save(source);
    return source;
  }

  async deleteSource(id: string): Promise<void> {
    const source = await this.sourceRepository.findById(id);
    if (!source) {
      throw new Error(`Source with id "${id}" not found`);
    }

    // TODO: Check if source is being used by any epochs
    // This would require checking EpochIndex table for references

    await this.sourceRepository.delete(id);
  }

  private validateConfiguration(type: DataSourceType, configuration: SourceConfiguration): void {
    switch (type) {
      case DataSourceType.S3:
        if (!configuration.bucket || !configuration.region) {
          throw new Error('S3 source requires bucket and region');
        }
        break;
      case DataSourceType.HTTP:
        if (!configuration.host || !configuration.path) {
          throw new Error('HTTP source requires host and path');
        }
        break;
      case DataSourceType.FILESYSTEM:
        if (!configuration.basePath) {
          throw new Error('Filesystem source requires basePath');
        }
        break;
      default:
        throw new Error(`Unknown source type: ${type}`);
    }
  }

  async testSourceConnection(id: string): Promise<{ success: boolean; error?: string }> {
    const source = await this.sourceRepository.findById(id);
    if (!source) {
      throw new Error(`Source with id "${id}" not found`);
    }

    // TODO: Implement actual connection testing based on source type
    // This would involve creating a DataSource instance and testing connectivity
    
    return { success: true };
  }
}
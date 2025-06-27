import { Entity } from '@/lib/domain/shared/interfaces/entity';
import { DataSourceType } from '@/generated/prisma';

export interface SourceConfiguration {
  [key: string]: unknown;
}

export interface S3Configuration extends SourceConfiguration {
  bucket: string;
  region: string;
  endpoint?: string;
  accessKeyId?: string;
  secretAccessKey?: string;
}

export interface HTTPConfiguration extends SourceConfiguration {
  host: string;
  path: string;
  username?: string;
  password?: string;
}

export interface FilesystemConfiguration extends SourceConfiguration {
  basePath: string;
}

export class Source implements Entity<Source> {
  constructor(
    public readonly id: string,
    public name: string,
    public type: DataSourceType,
    public configuration: SourceConfiguration,
    public enabled: boolean,
    public readonly createdAt: Date,
    public updatedAt: Date
  ) {}

  public equals(other: Entity<Source>): boolean {
    if (!(other instanceof Source)) {
      return false;
    }
    return this.id === other.id;
  }

  public enable(): void {
    this.enabled = true;
    this.updatedAt = new Date();
  }

  public disable(): void {
    this.enabled = false;
    this.updatedAt = new Date();
  }

  public updateConfiguration(configuration: SourceConfiguration): void {
    this.configuration = configuration;
    this.updatedAt = new Date();
  }

  public updateName(name: string): void {
    this.name = name;
    this.updatedAt = new Date();
  }

  public getTypedConfiguration(): S3Configuration | HTTPConfiguration | FilesystemConfiguration {
    switch (this.type) {
      case DataSourceType.S3:
        return this.configuration as S3Configuration;
      case DataSourceType.HTTP:
        return this.configuration as HTTPConfiguration;
      case DataSourceType.FILESYSTEM:
        return this.configuration as FilesystemConfiguration;
      default:
        throw new Error(`Unknown source type: ${this.type}`);
    }
  }

  public static create(
    name: string,
    type: DataSourceType,
    configuration: SourceConfiguration,
    enabled: boolean = true
  ): Source {
    const now = new Date();
    const id = `src_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    return new Source(id, name, type, configuration, enabled, now, now);
  }
}
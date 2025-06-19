import { Source as PrismaSource, DataSourceType, Prisma } from '@/generated/prisma';
import { Source, SourceConfiguration } from '@/lib/domain/source/entities/source';
import { 
  SourceRepository, 
  SourceFilters, 
  PaginationOptions, 
  PaginatedResult 
} from '@/lib/domain/source/repositories/source-repository';
import { prisma } from '../persistence/prisma';

export class PrismaSourceRepository implements SourceRepository {
  private toDomainEntity(prismaSource: PrismaSource): Source {
    return new Source(
      prismaSource.id,
      prismaSource.name,
      prismaSource.type,
      prismaSource.configuration as SourceConfiguration,
      prismaSource.enabled,
      prismaSource.createdAt,
      prismaSource.updatedAt
    );
  }

  private toPrismaData(source: Source): Prisma.SourceCreateInput | Prisma.SourceUpdateInput {
    return {
      id: source.id,
      name: source.name,
      type: source.type,
      configuration: source.configuration as Prisma.JsonObject,
      enabled: source.enabled,
      createdAt: source.createdAt,
      updatedAt: source.updatedAt
    };
  }

  async findById(id: string): Promise<Source | null> {
    const prismaSource = await prisma.source.findUnique({
      where: { id }
    });

    return prismaSource ? this.toDomainEntity(prismaSource) : null;
  }

  async findByName(name: string): Promise<Source | null> {
    const prismaSource = await prisma.source.findUnique({
      where: { name }
    });

    return prismaSource ? this.toDomainEntity(prismaSource) : null;
  }

  async findAll(
    filters?: SourceFilters, 
    pagination?: PaginationOptions
  ): Promise<PaginatedResult<Source>> {
    const where: Prisma.SourceWhereInput = {};

    if (filters) {
      if (filters.type) {
        where.type = filters.type;
      }
      if (filters.enabled !== undefined) {
        where.enabled = filters.enabled;
      }
      if (filters.search) {
        where.name = {
          contains: filters.search,
          mode: 'insensitive'
        };
      }
    }

    const page = pagination?.page || 1;
    const pageSize = pagination?.pageSize || 10;
    const skip = (page - 1) * pageSize;

    const [sources, total] = await Promise.all([
      prisma.source.findMany({
        where,
        skip,
        take: pageSize,
        orderBy: { createdAt: 'desc' }
      }),
      prisma.source.count({ where })
    ]);

    return {
      items: sources.map(s => this.toDomainEntity(s)),
      total,
      page,
      pageSize,
      totalPages: Math.ceil(total / pageSize)
    };
  }

  async save(source: Source): Promise<void> {
    const data = this.toPrismaData(source);
    
    await prisma.source.upsert({
      where: { id: source.id },
      update: data as Prisma.SourceUpdateInput,
      create: data as Prisma.SourceCreateInput
    });
  }

  async delete(id: string): Promise<void> {
    await prisma.source.delete({
      where: { id }
    });
  }

  async exists(id: string): Promise<boolean> {
    const count = await prisma.source.count({
      where: { id }
    });
    return count > 0;
  }

  async existsByName(name: string): Promise<boolean> {
    const count = await prisma.source.count({
      where: { name }
    });
    return count > 0;
  }

  async countByType(type: DataSourceType): Promise<number> {
    return await prisma.source.count({
      where: { type }
    });
  }

  async findEnabled(): Promise<Source[]> {
    const sources = await prisma.source.findMany({
      where: { enabled: true },
      orderBy: { name: 'asc' }
    });

    return sources.map(s => this.toDomainEntity(s));
  }
}
import { EpochRepository } from '@/lib/domain/epoch/repositories/epoch-repository';
import { Epoch } from '@/lib/domain/epoch/entities/epoch';
import { EpochId } from '@/lib/domain/epoch/value-objects/epoch-id';
import { EpochStatus } from '@/lib/domain/epoch/value-objects/epoch-status';
import { EpochIndex } from '@/lib/domain/epoch/entities/epoch-index';
import { EpochGsfa } from '@/lib/domain/epoch/entities/epoch-gsfa';
import { ContentIdentifier } from '@/lib/domain/epoch/value-objects/content-identifier';
import { IndexType } from '@/lib/domain/epoch/value-objects/index-type';
import { PrismaClient, Epoch as PrismaEpoch, EpochIndex as PrismaEpochIndex, EpochGsfa as PrismaEpochGsfa, Prisma } from '@/generated/prisma';
import { prisma } from '../persistence/prisma';

export class PrismaEpochRepository implements EpochRepository {
  private prisma: PrismaClient;

  constructor(prismaClient?: PrismaClient) {
    this.prisma = prismaClient || prisma;
  }

  async findById(id: EpochId): Promise<Epoch | null> {
    const epochData = await this.prisma.epoch.findUnique({
      where: { id: id.getValue() },
      include: {
        epochIndexes: true,
        epochGsfas: true
      }
    });

    if (!epochData) {
      return null;
    }

    return this.toDomain(epochData);
  }

  async findByEpochId(epochId: EpochId): Promise<Epoch | null> {
    return this.findById(epochId);
  }

  async findByEpochNumber(epochNumber: number): Promise<Epoch | null> {
    const epochData = await this.prisma.epoch.findUnique({
      where: { id: epochNumber },
      include: {
        epochIndexes: true,
        epochGsfas: true
      }
    });

    if (!epochData) {
      return null;
    }

    return this.toDomain(epochData);
  }

  async findAll(): Promise<Epoch[]> {
    const epochs = await this.prisma.epoch.findMany({
      include: {
        epochIndexes: true,
        epochGsfas: true
      },
      orderBy: { id: 'asc' }
    });

    return Promise.all(epochs.map(epoch => this.toDomain(epoch)));
  }

  async findWithPagination(params: {
    page: number;
    pageSize: number;
    status?: EpochStatus;
    search?: string;
  }): Promise<{ epochs: Epoch[]; totalCount: number }> {
    const { page, pageSize, status, search } = params;
    const skip = (page - 1) * pageSize;

    const where: Prisma.EpochWhereInput = {};

    if (status) {
      where.status = status.getValue();
    }

    if (search) {
      const searchId = parseInt(search, 10);
      if (!isNaN(searchId)) {
        where.OR = [
          { id: searchId },
          { epoch: { contains: search, mode: 'insensitive' } }
        ];
      } else {
        where.epoch = { contains: search, mode: 'insensitive' };
      }
    }

    const [epochs, totalCount] = await Promise.all([
      this.prisma.epoch.findMany({
        where,
        skip,
        take: pageSize,
        include: {
          epochIndexes: true,
          epochGsfas: true
        },
        orderBy: { id: 'asc' }
      }),
      this.prisma.epoch.count({ where })
    ]);

    const domainEpochs = await Promise.all(epochs.map(epoch => this.toDomain(epoch)));

    return {
      epochs: domainEpochs,
      totalCount
    };
  }

  async save(epoch: Epoch): Promise<void> {
    const id = epoch.getId().getValue();
    const status = epoch.getStatus().getValue();
    const cid = epoch.getCid()?.getValue() || null;

    await this.prisma.$transaction(async (tx) => {
      // Upsert the epoch
      await tx.epoch.upsert({
        where: { id },
        update: {
          status,
          cid,
          updatedAt: epoch.getUpdatedAt()
        },
        create: {
          id,
          epoch: id.toString(),
          status,
          cid,
          createdAt: epoch.getCreatedAt(),
          updatedAt: epoch.getUpdatedAt()
        }
      });

      // Delete existing indexes and re-insert
      await tx.epochIndex.deleteMany({
        where: { epoch: id.toString() }
      });

      const indexes = epoch.getIndexes();
      if (indexes.length > 0) {
        await tx.epochIndex.createMany({
          data: indexes.map(index => ({
            epoch: index.getEpochId().toString(),
            type: index.getType().getValue(),
            size: index.getSize(),
            status: index.getStatus(),
            location: index.getLocation(),
            sourceId: index.getSourceId()
          }))
        });
      }

      // Delete existing GSFA indexes and re-insert
      await tx.epochGsfa.deleteMany({
        where: { epoch: id.toString() }
      });

      const gsfaIndexes = epoch.getGsfaIndexes();
      if (gsfaIndexes.length > 0) {
        await tx.epochGsfa.createMany({
          data: gsfaIndexes.map(gsfa => ({
            id: gsfa.getId() || 0,
            epoch: gsfa.getEpochId().toString(),
            exists: gsfa.exists(),
            location: gsfa.getLocation()
          }))
        });
      }
    });

    // Mark events as committed after successful save
    epoch.markEventsAsCommitted();
  }

  async delete(id: EpochId): Promise<void> {
    await this.prisma.epoch.delete({
      where: { id: id.getValue() }
    });
  }

  async getLatestEpoch(): Promise<Epoch | null> {
    const epochData = await this.prisma.epoch.findFirst({
      orderBy: { id: 'desc' },
      include: {
        epochIndexes: true,
        epochGsfas: true
      }
    });

    if (!epochData) {
      return null;
    }

    return this.toDomain(epochData);
  }

  async findByStatus(status: EpochStatus): Promise<Epoch[]> {
    const epochs = await this.prisma.epoch.findMany({
      where: { status: status.getValue() },
      include: {
        epochIndexes: true,
        epochGsfas: true
      },
      orderBy: { id: 'asc' }
    });

    return Promise.all(epochs.map(epoch => this.toDomain(epoch)));
  }

  async findByRange(start: EpochId, end: EpochId): Promise<Epoch[]> {
    const epochs = await this.prisma.epoch.findMany({
      where: {
        id: {
          gte: start.getValue(),
          lte: end.getValue()
        }
      },
      include: {
        epochIndexes: true,
        epochGsfas: true
      },
      orderBy: { id: 'asc' }
    });

    return Promise.all(epochs.map(epoch => this.toDomain(epoch)));
  }

  async exists(epochId: EpochId): Promise<boolean> {
    const count = await this.prisma.epoch.count({
      where: { id: epochId.getValue() }
    });
    return count > 0;
  }

  private async toDomain(
    epochData: PrismaEpoch & {
      epochIndexes: PrismaEpochIndex[];
      epochGsfas: PrismaEpochGsfa[];
    }
  ): Promise<Epoch> {
    const epochId = new EpochId(epochData.id);
    const status = EpochStatus.fromString(epochData.status);
    const cid = epochData.cid ? new ContentIdentifier(epochData.cid) : undefined;

    const indexes = epochData.epochIndexes.map(indexData => 
      new EpochIndex({
        epochId,
        type: IndexType.fromString(indexData.type),
        size: indexData.size,
        status: indexData.status,
        location: indexData.location,
        sourceId: indexData.sourceId,
        createdAt: indexData.createdAt,
        updatedAt: indexData.updatedAt
      })
    );

    const gsfaIndexes = epochData.epochGsfas.map(gsfaData =>
      new EpochGsfa({
        id: gsfaData.id,
        epochId,
        exists: gsfaData.exists || false,
        location: gsfaData.location,
        createdAt: gsfaData.createdAt,
        updatedAt: gsfaData.updatedAt
      })
    );

    return new Epoch({
      id: epochId,
      status,
      cid,
      indexes,
      gsfaIndexes,
      createdAt: epochData.createdAt,
      updatedAt: epochData.updatedAt
    });
  }
}
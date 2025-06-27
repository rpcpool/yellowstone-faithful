import { prisma } from '@/lib/infrastructure/persistence/prisma';

export interface GetEpochsOptions {
  skip?: number;
  take?: number;
  orderBy?: 'asc' | 'desc';
}

export async function getEpochs(skip: number = 0, take: number = 100, options?: GetEpochsOptions) {
  return await prisma.epoch.findMany({
    skip,
    take,
    orderBy: { id: options?.orderBy || 'asc' },
  });
} 
import { prisma } from '@/lib/infrastructure/persistence/prisma';
import { EpochStatus } from './types';

export async function setStatus(id: number, status: EpochStatus) {
  // Check if the epoch exists
  const epoch = await prisma.epoch.findUnique({ where: { id } });
  if (!epoch) {
    return 'not_found';
  }

  return await prisma.epoch.update({ where: { id }, data: { status } });
} 
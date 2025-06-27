import { prisma } from '@/lib/infrastructure/persistence/prisma';
import { EpochStatus, IndexType } from './types';

// Update the status of an epoch based on available indexes
export async function updateEpochStatus(epochId: number): Promise<EpochStatus> {
  const epochIndexes = await prisma.epochIndex.findMany({
    where: {
      epoch: epochId.toString(),
    },
  });

  // Check for GSFA index
  const gsfaIndex = await prisma.epochGsfa.findFirst({
    where: {
      epoch: epochId.toString(),
      exists: true,
    },
  });

  const indexCounts = epochIndexes.reduce((acc, index) => {
    acc[index.type] = (acc[index.type] || 0) + 1;
    return acc;
  }, {} as Record<IndexType, number>);

  // Get all possible index types
  const allIndexTypes = Object.values(IndexType);
  const hasAllRegularIndexes = allIndexTypes.every(type => (indexCounts[type] || 0) > 0);
  const hasSomeRegularIndexes = Object.values(indexCounts).some(count => count > 0);
  const hasGsfaIndex = !!gsfaIndex;

  console.log(`[determineIndexStatus] Status analysis: hasAllRegularIndexes=${hasAllRegularIndexes}, hasGsfaIndex=${hasGsfaIndex}`);

  // If we have no sources for any index type, we are Not Processed

  let status: EpochStatus;

  // If we have no sources for any index type, we are Not Processed
  if (!hasSomeRegularIndexes) {
    status = EpochStatus.NotProcessed;
  }
  // If we have all regular indexes and a GSFA index, we are complete
  else if (hasAllRegularIndexes && hasGsfaIndex) {
    status = EpochStatus.Complete;
  }
  // If we have all regular indexes but no GSFA index, we are indexed
  else if (hasAllRegularIndexes) {
    status = EpochStatus.Indexed;
  }
  // If we have some but not all regular indexes, we are processing
  else {
    status = EpochStatus.Processing;
  }

  // Update the status of the epoch
  await prisma.epoch.update({
    where: { id: epochId },
    data: { status },
  });

  console.log(`[updateEpochStatus] Updated status of epoch ${epochId} to ${status}`);

  return status;
} 
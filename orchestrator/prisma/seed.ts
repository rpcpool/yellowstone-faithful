import "dotenv/config";

import { DataSourceType, EpochStatus, PrismaClient } from '../src/generated/prisma/index.js';
import { getEpochInfo } from '../src/lib/solana.js';

const prisma = new PrismaClient();

async function main() {
  const epochInfo = await getEpochInfo();
  const currentEpoch = epochInfo.result.epoch;

  const data = Array.from({ length: currentEpoch + 1 }, (_, i) => ({
    id: i,
    epoch: i.toString(),
    status: EpochStatus.NotProcessed,
  }));

  // Upsert to avoid duplicates if run multiple times
  for (const row of data) {
    await prisma.epoch.upsert({
      where: { id: row.id },
      update: {},
      create: row,
    });
  }

  // Seed the HTTP source for the old-faithful.net archive
  await prisma.source.upsert({
    where: { name: 'old-faithful.net' },
    update: {
      configuration: {
        host: 'https://files.old-faithful.net',
        path: '',
      },
      enabled: true,
    },
    create: {
      name: 'old-faithful.net',
      type: DataSourceType.HTTP,
      configuration: {
        host: 'https://files.old-faithful.net',
        path: '',
      },
      enabled: true,
    },
  });
}

main()
  .catch((e) => {
    console.error(e);
    process.exit(1);
  })
  .finally(async () => {
    await prisma.$disconnect();
  });

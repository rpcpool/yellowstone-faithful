import { EpochStatus, PrismaClient } from '../src/generated/prisma/index.js';
import { getEpochInfo } from '../src/lib/solana.js';

// Polyfill fetch for Node.js < 18
if (typeof fetch === 'undefined') {
  // @ts-ignore
  global.fetch = (...args) => import('node-fetch').then(({default: fetch}) => fetch(...args));
}

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
}

main()
  .catch((e) => {
    console.error(e);
    process.exit(1);
  })
  .finally(async () => {
    await prisma.$disconnect();
  });

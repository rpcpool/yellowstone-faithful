import { PrismaClient } from '@/generated/prisma';
import { getLatestEpoch } from '@/lib/epochs/get-latest-epoch';
import { NextResponse } from 'next/server';

const prisma = new PrismaClient();

export async function GET() {
  try {
    // Fetch current epoch from Solana RPC
    let currentEpoch: number;
    try {
      currentEpoch = await getLatestEpoch();
    } catch (error) {
      console.error('Failed to fetch current epoch:', error);
      // Fallback to a reasonable default if RPC fails
      currentEpoch = 800;
    }

    // Get the sum of all index sizes using Prisma aggregation
    const result = await prisma.epochIndex.aggregate({
      _sum: {
        size: true
      },
      _count: {
        id: true
      }
    });

    // Get status distribution across all epochs (not indexes)
    const statusCounts = await prisma.epoch.groupBy({
      by: ['status'],
      _count: {
        status: true
      }
    });

    // Get type distribution across all indexes
    const typeCounts = await prisma.epochIndex.groupBy({
      by: ['type'],
      _count: {
        type: true
      }
    });

    // Get type size distribution - sum of sizes by type
    const typeSizes = await prisma.epochIndex.groupBy({
      by: ['type'],
      _sum: {
        size: true
      }
    });

    // Get source distribution across all indexes
    const sourceCounts = await prisma.epochIndex.groupBy({
      by: ['sourceId'],
      _count: {
        sourceId: true
      }
    });

    // Get count of epochs with GSFA indexes
    const gsfaEpochCount = await prisma.epochGsfa.count({
      where: {
        exists: true
      }
    });

    // Get count of all epochs in the database
    const totalEpochsDb = await prisma.epoch.count();

    // Convert the grouped results to a more convenient format
    const statusDistribution = statusCounts.reduce((acc, item) => {
      acc[item.status] = item._count.status;
      return acc;
    }, {} as Record<string, number>);

    const typeDistribution = typeCounts.reduce((acc, item) => {
      acc[item.type] = item._count.type;
      return acc;
    }, {} as Record<string, number>);

    const typeSizeDistribution = typeSizes.reduce((acc, item) => {
      // Convert BigInt to number for JSON serialization
      acc[item.type] = Number(item._sum.size || 0);
      return acc;
    }, {} as Record<string, number>);

    // Get source names for the distribution
    const sourceIds = sourceCounts.map(s => s.sourceId);
    const sources = await prisma.source.findMany({
      where: { id: { in: sourceIds } }
    });
    const sourceNameMap = Object.fromEntries(sources.map(s => [s.id, s.name]));
    
    const sourceDistribution = sourceCounts.reduce((acc, item) => {
      const sourceName = sourceNameMap[item.sourceId] || item.sourceId;
      acc[sourceName] = item._count.sourceId || 0;
      return acc;
    }, {} as Record<string, number>);

    return NextResponse.json({
      totalSize: result._sum.size?.toString() || '0', // Convert BigInt to string
      totalIndexes: result._count.id,
      gsfaEpochCount, // New field for epochs with GSFA indexes
      currentEpoch, // Current epoch from Solana RPC
      totalEpochsDb, // New field: total epochs in DB
      statusDistribution,
      typeDistribution,
      typeSizeDistribution, // New field with storage sizes by type
      sourceDistribution
    });
  } catch (error) {
    console.error('Error fetching stats:', error);
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 });
  }
} 
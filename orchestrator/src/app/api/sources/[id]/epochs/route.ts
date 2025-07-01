import { NextResponse } from 'next/server';
import { PrismaClient } from '@/generated/prisma';

const prisma = new PrismaClient();

interface EpochWithIndexes {
  id: number;
  status: string;
  epochIndexes: {
    type: string;
    updatedAt: Date;
    sourceId: string;
  }[];
  updatedAt: Date;
}

export async function GET(
  _req: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  
  try {
    // First verify the source exists
    const source = await prisma.source.findUnique({
      where: { id }
    });

    if (!source) {
      return NextResponse.json(
        { error: 'Source not found' },
        { status: 404 }
      );
    }

    // Get all epochs that have indexes from this source
    const epochsWithIndexes = await prisma.epoch.findMany({
      where: {
        epochIndexes: {
          some: {
            sourceId: id
          }
        }
      },
      include: {
        epochIndexes: {
          where: {
            sourceId: id
          },
          select: {
            type: true,
            updatedAt: true,
            sourceId: true
          }
        }
      },
      orderBy: {
        id: 'desc'
      }
    });

    // Calculate status for each epoch based on available indexes from this source
    const epochs = epochsWithIndexes.map((epoch: EpochWithIndexes) => {
      // Get the index types available from this source
      const indexTypes = epoch.epochIndexes.map(idx => idx.type);
      
      // Calculate status based on what this source provides
      let status = 'NotProcessed';
      if (indexTypes.length > 0) {
        const hasAllStandardIndexes = [
          'CidToOffsetAndSize',
          'SigExists', 
          'SigToCid',
          'SlotToBlocktime',
          'SlotToCid'
        ].every(type => indexTypes.includes(type));
        
        const hasGsfaIndex = indexTypes.includes('Gsfa');
        
        if (hasAllStandardIndexes && hasGsfaIndex) {
          status = 'Complete';
        } else if (hasAllStandardIndexes) {
          status = 'Indexed';
        } else {
          status = 'Processing';
        }
      }

      // Get the most recent update time from this source's indexes
      const lastUpdated = epoch.epochIndexes.length > 0
        ? epoch.epochIndexes.reduce((latest, idx) => 
            idx.updatedAt > latest ? idx.updatedAt : latest, 
            epoch.epochIndexes[0].updatedAt
          )
        : epoch.updatedAt;

      return {
        epochNumber: epoch.id,
        status,
        indexes: epoch.epochIndexes.map(idx => ({
          type: idx.type,
          updatedAt: idx.updatedAt.toISOString()
        })),
        lastUpdated: lastUpdated.toISOString()
      };
    });

    // Calculate statistics
    const statistics = {
      total: epochs.length,
      byStatus: epochs.reduce((acc: Record<string, number>, epoch: { status: string }) => {
        acc[epoch.status] = (acc[epoch.status] || 0) + 1;
        return acc;
      }, {})
    };

    // Ensure all status types are represented in statistics
    ['NotProcessed', 'Processing', 'Indexed', 'Complete'].forEach(status => {
      if (!statistics.byStatus[status]) {
        statistics.byStatus[status] = 0;
      }
    });

    return NextResponse.json({ epochs, statistics });

  } catch (error) {
    console.error('Error fetching source epochs:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Internal server error' },
      { status: 500 }
    );
  } finally {
    await prisma.$disconnect();
  }
}
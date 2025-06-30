import { PrismaClient } from '@/generated/prisma';
import { NextResponse } from 'next/server';

const prisma = new PrismaClient();

export async function GET(
  req: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  const epochId = parseInt(id, 10);
  
  if (isNaN(epochId) || epochId < 0) {
    return NextResponse.json({ error: 'Invalid epoch ID' }, { status: 400 });
  }

  try {
    // Get the epoch
    const epoch = await prisma.epoch.findUnique({ 
      where: { id: epochId } 
    });

    if (!epoch) {
      return NextResponse.json({ error: 'Epoch not found' }, { status: 404 });
    }

    // Get all indexes for this epoch
    const indexes = await prisma.epochIndex.findMany({
      where: { epoch: epoch.epoch },
      orderBy: { type: 'asc' }
    });

    // Get GSFA index for this epoch
    const gsfaIndex = await prisma.epochGsfa.findFirst({
      where: { epoch: epoch.epoch }
    });

    // Calculate stats
    const totalSize = indexes.reduce((sum, index) => sum + Number(index.size), 0);
    const statusCounts = indexes.reduce((acc, index) => {
      acc[index.status] = (acc[index.status] || 0) + 1;
      return acc;
    }, {} as Record<string, number>);

    const typeCounts = indexes.reduce((acc, index) => {
      acc[index.type] = (acc[index.type] || 0) + 1;
      return acc;
    }, {} as Record<string, number>);

    return NextResponse.json({
      epoch: {
        id: epoch.id,
        epoch: epoch.epoch,
        status: epoch.status,
        createdAt: epoch.createdAt,
        updatedAt: epoch.updatedAt
      },
      indexes: indexes.map(index => ({
        ...index,
        size: index.size.toString() // Convert BigInt to string
      })),
      gsfa: gsfaIndex ? {
        id: gsfaIndex.id,
        epoch: gsfaIndex.epoch,
        exists: gsfaIndex.exists,
        location: gsfaIndex.location,
        createdAt: gsfaIndex.createdAt,
        updatedAt: gsfaIndex.updatedAt
      } : null,
      stats: {
        totalIndexes: indexes.length,
        totalSize,
        statusCounts,
        typeCounts
      }
    });
  } catch (error) {
    console.error('Error fetching epoch details:', error);
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 });
  }
} 
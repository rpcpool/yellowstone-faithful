import { NextRequest, NextResponse } from 'next/server';
import { IndexType, Prisma } from '../../../generated/prisma/index.js';
import { prisma } from '../../../lib/prisma';

// Helper function to convert BigInt values to numbers
function convertBigIntToNumber(obj: unknown): unknown {
  if (obj === null || obj === undefined) {
    return obj;
  }
  
  if (typeof obj === 'bigint') {
    return Number(obj);
  }
  
  if (Array.isArray(obj)) {
    return obj.map(convertBigIntToNumber);
  }
  
  if (typeof obj === 'object') {
    const converted: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(obj)) {
      converted[key] = convertBigIntToNumber(value);
    }
    return converted;
  }
  
  return obj;
}

export async function GET(req: NextRequest) {
  try {
    const { searchParams } = new URL(req.url);
    const page = parseInt(searchParams.get('page') || '1', 10);
    const pageSize = parseInt(searchParams.get('pageSize') || '20', 10);
    const sourceParam = searchParams.get('source');
    const typeParam = searchParams.get('type');
    const searchParam = searchParams.get('search');
    const sortBy = searchParams.get('sortBy') || 'createdAt';
    const sortOrder = searchParams.get('sortOrder') || 'desc';

    if (isNaN(page) || isNaN(pageSize) || page < 1 || pageSize < 1 || pageSize > 100) {
      return NextResponse.json({ success: false, error: 'Invalid pagination parameters' }, { status: 400 });
    }

    // Validate sort parameters
    const validSortFields = ['id', 'epoch', 'type', 'source', 'size', 'status', 'createdAt', 'updatedAt'];
    const validSortOrders = ['asc', 'desc'];
    
    if (!validSortFields.includes(sortBy) || !validSortOrders.includes(sortOrder)) {
      return NextResponse.json({ success: false, error: 'Invalid sort parameters' }, { status: 400 });
    }

    const where: Prisma.EpochIndexWhereInput = {};
    if (sourceParam) {
      where.source = {
        name: sourceParam
      };
    }
    if (typeParam) where.type = typeParam as IndexType;

    if (searchParam) {
      const searchId = parseInt(searchParam, 10);
      if (!isNaN(searchId)) {
        where.OR = [
          { id: searchId },
          { epoch: { contains: searchParam, mode: 'insensitive' } },
        ];
      } else {
        where.epoch = { contains: searchParam, mode: 'insensitive' };
      }
    }

    const skip = (page - 1) * pageSize;

    // Build orderBy object
    const orderBy: Prisma.EpochIndexOrderByWithRelationInput = {
      [sortBy]: sortOrder as Prisma.SortOrder
    };

    const [indexes, totalCount, sourceCounts, typeCounts] = await Promise.all([
      prisma.epochIndex.findMany({
        where,
        orderBy,
        skip,
        take: pageSize,
        include: {
          source: true
        }
      }),
      prisma.epochIndex.count({ where }),
      prisma.epochIndex.groupBy({ by: ['sourceId'], _count: { sourceId: true } }),
      prisma.epochIndex.groupBy({ by: ['type'], _count: { type: true } }),
    ]);

    // Get source names for the source counts
    const sourceIds = sourceCounts.map(s => s.sourceId);
    const sources = await prisma.source.findMany({
      where: { id: { in: sourceIds } }
    });
    const sourceNameMap = Object.fromEntries(sources.map(s => [s.id, s.name]));
    const availableSources = sourceCounts.map((s) => sourceNameMap[s.sourceId] || s.sourceId);
    const availableTypes = typeCounts.map((t) => t.type);

    return NextResponse.json({
      success: true,
      indexes: convertBigIntToNumber(indexes),
      filters: {
        source: sourceParam || null,
        type: typeParam || null,
        search: searchParam || null,
      },
      availableSources,
      availableTypes,
      pagination: {
        page,
        pageSize,
        totalCount: convertBigIntToNumber(totalCount),
        totalPages: Math.ceil(Number(totalCount) / pageSize),
      },
    });
  } catch (error) {
    console.error('Error fetching indexes:', error);
    return NextResponse.json({ success: false, error: 'Internal server error' }, { status: 500 });
  }
}

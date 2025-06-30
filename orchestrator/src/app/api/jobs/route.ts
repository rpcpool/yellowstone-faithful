import { Prisma } from '@/generated/prisma';
import { prisma } from '@/lib/prisma';
import { NextRequest, NextResponse } from 'next/server';

export async function GET(req: NextRequest) {
  try {
    const { searchParams } = new URL(req.url);
    const page = parseInt(searchParams.get('page') || '1', 10);
    const pageSize = parseInt(searchParams.get('pageSize') || '20', 10);
    const epochIdParam = searchParams.get('epochId');
    const jobTypeParam = searchParams.get('jobType');

    if (isNaN(page) || isNaN(pageSize) || page < 1 || pageSize < 1 || pageSize > 100) {
      return NextResponse.json({ success: false, error: 'Invalid pagination parameters' }, { status: 400 });
    }

    const skip = (page - 1) * pageSize;

    // Build where clause based on query parameters
    const whereClause: Prisma.JobWhereInput = {};
    
    if (epochIdParam) {
      const epochId = parseInt(epochIdParam, 10);
      if (isNaN(epochId)) {
        return NextResponse.json({ success: false, error: 'Invalid epochId parameter' }, { status: 400 });
      }
      whereClause.epochId = epochId;
    }

    if (jobTypeParam) {
      whereClause.jobType = jobTypeParam;
    }

    const [jobs, totalCount] = await Promise.all([
      prisma.job.findMany({
        where: whereClause,
        orderBy: { createdAt: 'desc' },
        skip,
        take: pageSize,
      }),
      prisma.job.count({ where: whereClause }),
    ]);

    return NextResponse.json({
      success: true,
      jobs: jobs.map(job => ({
        id: job.id,
        epochId: job.epochId,
        jobType: job.jobType,
        status: job.status,
        createdAt: job.createdAt,
        updatedAt: job.updatedAt,
        metadata: job.metadata,
      })),
      filters: {
        epochId: epochIdParam ? parseInt(epochIdParam, 10) : null,
        jobType: jobTypeParam || null,
      },
      pagination: {
        page,
        pageSize,
        totalCount,
        totalPages: Math.ceil(totalCount / pageSize),
      },
    });
  } catch (error) {
    console.error('Error fetching jobs:', error);
    return NextResponse.json(
      { success: false, error: error instanceof Error ? error.message : 'Unknown error' },
      { status: 500 },
    );
  }
}

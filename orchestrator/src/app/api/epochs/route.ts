import { NextResponse } from 'next/server';
import { PrismaEpochRepository } from '@/lib/infrastructure/repositories/prisma-epoch-repository';
import { GetEpochsUseCase } from '@/lib/application/epoch/use-cases/get-epochs';
import { GetEpochByIdUseCase } from '@/lib/application/epoch/use-cases/get-epoch-by-id';
import { GetEpochsRangeUseCase } from '@/lib/application/epoch/use-cases/get-epochs-range';
import { Epoch } from '@/lib/domain/epoch/entities/epoch';
import { EpochId } from '@/lib/domain/epoch/value-objects/epoch-id';
import { EpochStatus } from '@/lib/domain/epoch/value-objects/epoch-status';

// Initialize repository
const epochRepository = new PrismaEpochRepository();

export async function GET(req: Request) {
  try {
    const { searchParams } = new URL(req.url);
    const epochParam = searchParams.get('epoch');
    const startParam = searchParams.get('start');
    const endParam = searchParams.get('end');
    const pageParam = searchParams.get('page');
    const pageSizeParam = searchParams.get('pageSize');

    // Pagination query: /api/epochs?page=1&pageSize=20&search=term&status=Complete
    if (pageParam && pageSizeParam) {
      const page = parseInt(pageParam, 10);
      const pageSize = parseInt(pageSizeParam, 10);
      const searchParam = searchParams.get('search');
      const statusParam = searchParams.get('status');
      
      const useCase = new GetEpochsUseCase(epochRepository);
      const result = await useCase.execute({
        page,
        pageSize,
        search: searchParam || undefined,
        status: statusParam || undefined
      });

      return NextResponse.json(result);
    }

    // Range query: /api/epochs?start=10&end=20
    if (startParam && endParam) {
      const start = parseInt(startParam, 10);
      const end = parseInt(endParam, 10);
      
      const useCase = new GetEpochsRangeUseCase(epochRepository);
      const result = await useCase.execute({ start, end });
      
      // Transform to match legacy format
      const epochStatus = result.epochs.map(epochDto => {
        const objects = epochDto.indexes && epochDto.indexes.length > 0 
          ? epochDto.indexes.map(index => ({
              name: `${index.type}-${epochDto.id}`,
              size: Number(index.size),
              status: index.status,
              location: index.location
            }))
          : [{ name: `epoch-${epochDto.id}`, size: 0, status: epochDto.status }];
        
        return {
          hasData: true,
          objects,
          epochStatus: epochDto.status
        };
      });
      
      return NextResponse.json({ epochs: epochStatus });
    }

    // Single epoch query: /api/epochs?epoch=5
    if (epochParam) {
      const epochId = parseInt(epochParam, 10);
      if (isNaN(epochId) || epochId < 0) {
        return NextResponse.json({ error: 'Invalid epoch' }, { status: 400 });
      }
      
      const useCase = new GetEpochByIdUseCase(epochRepository);
      const result = await useCase.execute({ epochId });
      
      if (!result.epoch) {
        return NextResponse.json({ error: 'Epoch not found' }, { status: 404 });
      }
      
      // Transform to match legacy format
      const epochDto = result.epoch;
      const objects = epochDto.indexes && epochDto.indexes.length > 0 
        ? epochDto.indexes.map(index => ({
            name: `${index.type}-${epochDto.id}`,
            size: Number(index.size),
            status: index.status,
            location: index.location
          }))
        : [{ name: `epoch-${epochDto.id}`, size: 0, status: epochDto.status }];
      
      return NextResponse.json({ 
        epoch: { 
          hasData: true, 
          objects, 
          epochStatus: epochDto.status 
        } 
      });
    }

    // All epochs (fallback - limited to 100)
    const useCase = new GetEpochsUseCase(epochRepository);
    const result = await useCase.execute({
      page: 1,
      pageSize: 100
    });
    
    // Transform to match legacy format
    const epochStatus = result.epochs.map(epochDto => {
      const objects = epochDto.indexes && epochDto.indexes.length > 0 
        ? epochDto.indexes.map(index => ({
            name: `${index.type}-${epochDto.id}`,
            size: Number(index.size),
            status: index.status,
            location: index.location
          }))
        : [{ name: `epoch-${epochDto.id}`, size: 0, status: epochDto.status }];
      
      return {
        hasData: true,
        objects,
        epochStatus: epochDto.status
      };
    });
    
    return NextResponse.json({ epochs: epochStatus });
  } catch (error) {
    console.error('Error in epochs API:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Internal server error' },
      { status: 500 }
    );
  }
}

export async function POST(req: Request) {
  try {
    const body = await req.json();
    const { epochs, startEpoch, endEpoch } = body;

    // Validate input
    if (!epochs && (!startEpoch || !endEpoch)) {
      return NextResponse.json(
        { error: 'Must provide either epochs array or startEpoch/endEpoch range' },
        { status: 400 }
      );
    }

    let epochNumbers: number[] = [];

    // Handle single epochs array
    if (epochs && Array.isArray(epochs)) {
      epochNumbers = epochs.filter(e => typeof e === 'number' && e >= 0);
    }

    // Handle range
    if (startEpoch !== undefined && endEpoch !== undefined) {
      const start = parseInt(startEpoch, 10);
      const end = parseInt(endEpoch, 10);
      
      if (isNaN(start) || isNaN(end) || start < 0 || end < 0 || start > end) {
        return NextResponse.json(
          { error: 'Invalid epoch range' },
          { status: 400 }
        );
      }

      // Limit range to prevent too many epochs at once
      if (end - start > 1000) {
        return NextResponse.json(
          { error: 'Range too large. Maximum 1000 epochs at a time' },
          { status: 400 }
        );
      }

      for (let i = start; i <= end; i++) {
        epochNumbers.push(i);
      }
    }

    if (epochNumbers.length === 0) {
      return NextResponse.json(
        { error: 'No valid epochs to add' },
        { status: 400 }
      );
    }

    // Create epochs in database
    let created = 0;
    let skipped = 0;
    
    for (const epochNumber of epochNumbers) {
      try {
        // Check if epoch already exists
        const exists = await epochRepository.exists(new EpochId(epochNumber));
        
        if (!exists) {
          // Create new epoch with NotProcessed status
          const epoch = new Epoch({
            id: new EpochId(epochNumber),
            status: EpochStatus.NotProcessed(),
            indexes: [],
            gsfaIndexes: [],
            createdAt: new Date(),
            updatedAt: new Date()
          });
          
          await epochRepository.save(epoch);
          created++;
        } else {
          skipped++;
        }
      } catch (error) {
        console.error(`Error creating epoch ${epochNumber}:`, error);
      }
    }

    return NextResponse.json({
      success: true,
      message: `Created ${created} epochs, skipped ${skipped} existing epochs`,
      created,
      skipped,
      total: epochNumbers.length
    });

  } catch (error) {
    console.error('Error in epochs POST API:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Internal server error' },
      { status: 500 }
    );
  }
}
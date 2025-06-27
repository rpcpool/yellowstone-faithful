import { NextResponse } from 'next/server';
import { PrismaSourceRepository } from '@/lib/infrastructure/repositories/prisma-source-repository';
import { SourceDomainService } from '@/lib/domain/source/services/source-domain-service';
import { CreateSourceUseCase } from '@/lib/application/source/use-cases/create-source';
import { UpdateSourceUseCase } from '@/lib/application/source/use-cases/update-source';
import { DataSourceType } from '@/generated/prisma';

// Initialize repositories and services
const sourceRepository = new PrismaSourceRepository();
const sourceDomainService = new SourceDomainService(sourceRepository);

interface WorkerRegistrationRequest {
  hostname: string;
  pid: number;
  capabilities: string[];
}

export async function POST(req: Request) {
  try {
    const body: WorkerRegistrationRequest = await req.json();
    
    // Validate required fields
    if (!body.hostname || !body.pid || !body.capabilities) {
      return NextResponse.json(
        { error: 'Missing required fields: hostname, pid, capabilities' },
        { status: 400 }
      );
    }

    // Generate worker name (one worker per hostname)
    const workerName = `worker-${body.hostname}`;
    
    // Check if worker already exists
    const existingSource = await sourceRepository.findByName(workerName);
    if (existingSource) {
      // Worker already registered, update configuration with new PID and return 200
      const updatedConfiguration = {
        ...existingSource.configuration,
        pid: body.pid,
        capabilities: body.capabilities,
        startedAt: new Date().toISOString()
      } as any;
      
      // Update the source configuration
      const updateUseCase = new UpdateSourceUseCase(sourceDomainService);
      const updatedResult = await updateUseCase.execute({
        id: existingSource.id,
        configuration: updatedConfiguration
      });
      
      return NextResponse.json(
        { 
          message: 'Worker already registered (configuration updated)',
          workerId: existingSource.id,
          source: updatedResult.source
        },
        { status: 200 }
      );
    }

    // Create worker configuration for FILESYSTEM type
    const configuration = {
      basePath: `/workers/${body.hostname}`, // Required for FILESYSTEM validation
      isWorker: true,
      hostname: body.hostname,
      pid: body.pid,
      capabilities: body.capabilities,
      startedAt: new Date().toISOString()
    };

    // Create new worker source
    const useCase = new CreateSourceUseCase(sourceDomainService);
    const result = await useCase.execute({
      name: workerName,
      type: DataSourceType.FILESYSTEM,
      configuration,
      enabled: true
    });

    return NextResponse.json(
      {
        message: 'Worker registered successfully',
        workerId: result.source.id,
        source: result.source
      },
      { status: 201 }
    );
  } catch (error) {
    console.error('Error in worker registration:', error);
    
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Internal server error' },
      { status: 500 }
    );
  }
}
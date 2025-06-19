import { NextResponse } from 'next/server';
import { PrismaSourceRepository } from '@/lib/infrastructure/repositories/prisma-source-repository';
import { SourceDomainService } from '@/lib/domain/source/services/source-domain-service';
import { GetSourcesUseCase } from '@/lib/application/source/use-cases/get-sources';
import { CreateSourceUseCase } from '@/lib/application/source/use-cases/create-source';
import { DataSourceType } from '@/generated/prisma';

// Initialize repositories and services
const sourceRepository = new PrismaSourceRepository();
const sourceDomainService = new SourceDomainService(sourceRepository);

export async function GET(req: Request) {
  try {
    const { searchParams } = new URL(req.url);
    const page = parseInt(searchParams.get('page') || '1', 10);
    const pageSize = parseInt(searchParams.get('pageSize') || '10', 10);
    const type = searchParams.get('type') as DataSourceType | undefined;
    const enabled = searchParams.get('enabled') 
      ? searchParams.get('enabled') === 'true' 
      : undefined;
    const search = searchParams.get('search') || undefined;

    const useCase = new GetSourcesUseCase(sourceRepository);
    const result = await useCase.execute({
      page,
      pageSize,
      type,
      enabled,
      search
    });

    return NextResponse.json(result);
  } catch (error) {
    console.error('Error in sources GET:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Internal server error' },
      { status: 500 }
    );
  }
}

export async function POST(req: Request) {
  try {
    const body = await req.json();
    
    if (!body.name || !body.type || !body.configuration) {
      return NextResponse.json(
        { error: 'Missing required fields: name, type, configuration' },
        { status: 400 }
      );
    }

    // Validate source type
    if (!Object.values(DataSourceType).includes(body.type)) {
      return NextResponse.json(
        { error: `Invalid source type. Must be one of: ${Object.values(DataSourceType).join(', ')}` },
        { status: 400 }
      );
    }

    const useCase = new CreateSourceUseCase(sourceDomainService);
    const result = await useCase.execute({
      name: body.name,
      type: body.type,
      configuration: body.configuration,
      enabled: body.enabled
    });

    return NextResponse.json(result, { status: 201 });
  } catch (error) {
    console.error('Error in sources POST:', error);
    
    if (error instanceof Error && error.message.includes('already exists')) {
      return NextResponse.json(
        { error: error.message },
        { status: 409 }
      );
    }

    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Internal server error' },
      { status: 500 }
    );
  }
}
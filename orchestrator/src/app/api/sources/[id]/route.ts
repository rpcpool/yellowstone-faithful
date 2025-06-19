import { NextResponse } from 'next/server';
import { PrismaSourceRepository } from '@/lib/infrastructure/repositories/prisma-source-repository';
import { SourceDomainService } from '@/lib/domain/source/services/source-domain-service';
import { GetSourceByIdUseCase } from '@/lib/application/source/use-cases/get-source-by-id';
import { UpdateSourceUseCase } from '@/lib/application/source/use-cases/update-source';
import { DeleteSourceUseCase } from '@/lib/application/source/use-cases/delete-source';

// Initialize repositories and services
const sourceRepository = new PrismaSourceRepository();
const sourceDomainService = new SourceDomainService(sourceRepository);

export async function GET(
  req: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  try {
    const useCase = new GetSourceByIdUseCase(sourceRepository);
    const result = await useCase.execute({ id });

    if (!result.source) {
      return NextResponse.json(
        { error: 'Source not found' },
        { status: 404 }
      );
    }

    return NextResponse.json(result);
  } catch (error) {
    console.error('Error in source GET:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Internal server error' },
      { status: 500 }
    );
  }
}

export async function PUT(
  req: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  try {
    const body = await req.json();

    const useCase = new UpdateSourceUseCase(sourceDomainService);
    const result = await useCase.execute({
      id,
      name: body.name,
      configuration: body.configuration,
      enabled: body.enabled
    });

    return NextResponse.json(result);
  } catch (error) {
    console.error('Error in source PUT:', error);
    
    if (error instanceof Error) {
      if (error.message.includes('not found')) {
        return NextResponse.json(
          { error: error.message },
          { status: 404 }
        );
      }
      if (error.message.includes('already exists')) {
        return NextResponse.json(
          { error: error.message },
          { status: 409 }
        );
      }
    }

    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Internal server error' },
      { status: 500 }
    );
  }
}

export async function DELETE(
  req: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  try {
    // TODO: Check if source has any associated epochs before deletion
    
    const useCase = new DeleteSourceUseCase(sourceDomainService);
    await useCase.execute({ id });

    return NextResponse.json({ success: true });
  } catch (error) {
    console.error('Error in source DELETE:', error);
    
    if (error instanceof Error && error.message.includes('not found')) {
      return NextResponse.json(
        { error: error.message },
        { status: 404 }
      );
    }

    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Internal server error' },
      { status: 500 }
    );
  }
}
import { prisma } from '@/lib/infrastructure/persistence/prisma';
import * as yaml from 'js-yaml';
import { NextRequest, NextResponse } from 'next/server';

// Convert PascalCase IndexType to snake_case for YAML keys
function indexTypeToSnakeCase(indexType: string): string {
  return indexType
    .replace(/([A-Z])/g, '_$1')
    .toLowerCase()
    .substring(1); // Remove leading underscore
}

export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ epochId: string }> }
) {
  const { epochId } = await params;
  const epochIdNum = parseInt(epochId, 10);
  
  if (isNaN(epochIdNum) || epochIdNum < 0) {
    return NextResponse.json({ error: 'Invalid epoch ID' }, { status: 400 });
  }

  try {
    // Get the epoch
    const epoch = await prisma.epoch.findUnique({ 
      where: { id: epochIdNum } 
    });

    if (!epoch) {
      return NextResponse.json({ error: 'Epoch not found' }, { status: 404 });
    }

    // Get all indexes for this epoch
    const indexes = await prisma.epochIndex.findMany({
      where: { epoch: epoch.epoch },
      orderBy: { type: 'asc' }
    });

    // Return 404 if no indexes are available for this epoch
    if (indexes.length === 0) {
      return NextResponse.json({ error: 'No indexes found for this epoch' }, { status: 404 });
    }

    // Get GSFA index for this epoch if it exists
    const gsfaIndex = await prisma.epochGsfa.findFirst({
      where: { 
        epoch: epoch.epoch,
        exists: true 
      }
    });

    // Build the YAML configuration
    const config = {
      version: 1,
      epoch: epochIdNum,
      data: {
        car: {
          uri: `https://files.old-faithful.net/${epochIdNum}/epoch-${epochIdNum}.car`
        }
      },
      indexes: {} as Record<string, { uri: string }>
    };

    // Source preference order: Local, HTTP, Old Faithful
    const sourcePreferenceOrder = ['Local', 'HTTP', 'Old Faithful'];

    // Group indexes by type
    const indexesByType = indexes.reduce((acc, index) => {
      if (!acc[index.type]) {
        acc[index.type] = [];
      }
      acc[index.type].push(index);
      return acc;
    }, {} as Record<string, typeof indexes>);

    // For each index type, select the best available source based on preference order
    for (const [indexType, indexList] of Object.entries(indexesByType)) {
      const indexKey = indexTypeToSnakeCase(indexType);
      let selectedIndex: typeof indexes[0] | null = null;

      // Try to find index from sources in preference order
      for (const preferredSource of sourcePreferenceOrder) {
        const indexFromSource = indexList.find(idx => idx.source === preferredSource);
        if (indexFromSource) {
          selectedIndex = indexFromSource;
          console.log(`Using ${preferredSource} source for ${indexType}: ${indexFromSource.location}`);
          break;
        }
      }

      // If no preferred source found, use the first available
      if (!selectedIndex && indexList.length > 0) {
        selectedIndex = indexList[0];
        console.log(`Using fallback source ${selectedIndex.source} for ${indexType}: ${selectedIndex.location}`);
      }

      if (selectedIndex) {
        config.indexes[indexKey] = { uri: selectedIndex.location };
      }
    }

    // Add GSFA index if it exists
    if (gsfaIndex) {
      config.indexes.gsfa = { uri: gsfaIndex.location };
    }

    // Convert to YAML
    const yamlContent = yaml.dump(config, {
      indent: 2,
      lineWidth: -1, // Unlimited line width to prevent automatic line breaks
      noRefs: true,
      flowLevel: -1, // Use flow style when needed to avoid block scalars
      quotingType: '"', // Force double quotes for strings
      forceQuotes: true // Force quotes for all strings to prevent block scalars
    });

    return new NextResponse(yamlContent, {
      status: 200,
      headers: {
        'Content-Type': 'application/x-yaml',
        'Cache-Control': 'public, max-age=3600' // Cache for 1 hour
      }
    });
  } catch (error) {
    console.error('Error generating epoch config:', error);
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 });
  }
}


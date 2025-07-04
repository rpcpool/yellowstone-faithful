import { NextResponse } from 'next/server';
import { getLatestEpoch } from '@/lib/epochs/get-latest-epoch';

export async function GET() {
  try {
    const currentEpoch = await getLatestEpoch();
    
    return NextResponse.json({
      epoch: currentEpoch,
      timestamp: new Date().toISOString()
    });
  } catch (error) {
    console.error('Failed to fetch current epoch:', error);
    
    // Return a service unavailable error
    return NextResponse.json(
      { 
        error: 'Failed to fetch current epoch from Solana RPC',
        details: error instanceof Error ? error.message : 'Unknown error'
      },
      { status: 503 }
    );
  }
}
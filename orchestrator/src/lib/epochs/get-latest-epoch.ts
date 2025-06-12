// Returns the latest epoch number from the Solana RPC API
export async function getLatestEpoch(rpcUrl: string = 'https://api.mainnet-beta.solana.com'): Promise<number> {
  type EpochInfoResponse = {
    jsonrpc: string;
    result: {
      epoch: number;
      absoluteSlot: number;
      blockHeight: number;
      slotIndex: number;
      slotsInEpoch: number;
      transactionCount: number | null;
    };
    id: number;
  };

  const body = JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'getEpochInfo',
  });

  const res = await fetch(rpcUrl, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
  });

  if (!res.ok) {
    throw new Error(`Failed to fetch epoch info: ${res.status} ${res.statusText}`);
  }

  const data = (await res.json()) as EpochInfoResponse;
  if (!data.result || typeof data.result.epoch !== 'number') {
    throw new Error('Invalid response from Solana RPC');
  }
  return data.result.epoch;
}
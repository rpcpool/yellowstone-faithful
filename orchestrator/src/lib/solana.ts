export type EpochInfoResponse = {
  jsonrpc: string;
  result: {
    absoluteSlot: number;
    blockHeight: number;
    epoch: number;
    slotIndex: number;
    slotsInEpoch: number;
    transactionCount: number;
  };
  id: number;
};

export async function getEpochInfo(): Promise<EpochInfoResponse> {
  const response = await fetch('https://api.mainnet-beta.solana.com', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'getEpochInfo' }),
  });
  const data = await response.json();
  return data;
}
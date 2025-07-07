export const getRandomSlotFromRange = (start: number, end: number): number => {
  if (start >= end) {
    throw new Error('Start must be less than end');
  }
  return Math.floor(Math.random() * (end - start + 1)) + start;
}

export const getRandomSlotFromEpoch = (epoch: number): number => {
  const slotsPerEpoch = 432000; // Rough estimate based on slots-per-epoch
  const startSlot = epoch * slotsPerEpoch;
  const endSlot = startSlot + slotsPerEpoch - 1;
  return Math.floor(Math.random() * (endSlot - startSlot + 1)) + startSlot;
}

export interface RPCError {
  code: number;
  message: string;
}

export interface RPCResponse {
  jsonrpc: string;
  id: number;
  error: RPCError | null;
  result?: any;
}

export const parseResponseBody = (body: string | ArrayBuffer): string => {
  if (body instanceof ArrayBuffer) {
    return new TextDecoder().decode(body);
  }
  return body;
}

export const getRandomSlot = (start?: number, end?: number): number => {
  const min = start || 0;
  const max = end || 1000000; // Default range if not provided
  return getRandomSlotFromRange(min, max);
}

// List of known active Solana addresses for testing
const KNOWN_ADDRESSES = [
  'EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v', // USDC Mint
  'DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263', // Bonk Mint
  'Stake11111111111111111111111111111111111111', // Stake Program
  'TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA', // Token Program
  'TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb', // Token 2022 Program
  'BPFLoaderUpgradeab1e11111111111111111111111', // BPF Loader Upgradeable

];

export const getRandomSolanaAddress = (): string => {
  // For testing purposes, we'll use known addresses to ensure we get results
  return KNOWN_ADDRESSES[Math.floor(Math.random() * KNOWN_ADDRESSES.length)];
}
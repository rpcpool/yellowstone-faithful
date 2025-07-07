// Base RPC types
export interface RPCRequest {
  jsonrpc: '2.0';
  id: number;
  method: string;
  params: any[];
}

export interface RPCResponse<T = any> {
  jsonrpc: string;
  id: number;
  error?: {
    code: number;
    message: string;
  };
  result?: T;
}

// Common types used across multiple responses
export interface Commitment {
  commitment: 'processed' | 'confirmed' | 'finalized';
  maxSupportedTransactionVersion: number;
}

export interface TransactionError {
  err: any;
}

// Specific response types
export interface BlockResponse {
  blockhash: string;
  parentSlot: number;
  previousBlockhash: string;
  transactions: Array<{
    transaction: any;
    meta: any;
  }>;
}

export interface TransactionResponse {
  slot: number;
  transaction: {
    message: {
      accountKeys: string[];
      instructions: any[];
      recentBlockhash: string;
    };
    signatures: string[];
  };
  meta: {
    err: TransactionError | null;
    fee: number;
    postBalances: number[];
    preBalances: number[];
    status: { Ok: null } | { Err: TransactionError };
  };
}

export interface SignatureResponse {
  signature: string;
  slot: number;
  err: TransactionError | null;
  memo: string | null;
  blockTime?: number;
}

export interface BlockTimeResponse {
  timestamp: number | null;
}

export interface SlotResponse {
  slot: number;
}

export interface VersionResponse {
  'solana-core': string;
  'feature-set': number;
}

export interface Address {
  name: string;
  address: string;
}

// Type the specific RPC responses
export type GetBlockRPCResponse = RPCResponse<BlockResponse>;
export type GetTransactionRPCResponse = RPCResponse<TransactionResponse>;
export type GetSignaturesForAddressRPCResponse = RPCResponse<SignatureResponse[]>;
export type GetBlockTimeRPCResponse = RPCResponse<BlockTimeResponse>;
export type GetSlotRPCResponse = RPCResponse<SlotResponse>;
export type GetVersionRPCResponse = RPCResponse<VersionResponse>;
export type GetGenesisHashRPCResponse = RPCResponse<string>;
export type GetFirstAvailableBlockRPCResponse = RPCResponse<number>;

export interface SignatureInfo {
  signature: string;
  slot: number;
  err: any;
  memo: string | null;
  blockTime: number | null;
  confirmationStatus: 'processed' | 'confirmed' | 'finalized';
}

export type GetSignaturesForAddressResponse = RPCResponse<SignatureInfo[]>;

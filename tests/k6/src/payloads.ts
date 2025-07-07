import { Commitment, RPCRequest } from './types';

export const getBlockPayload = (
  slot: number,
  commitment: Commitment['commitment'] = 'finalized',
  maxSupportedTransactionVersion: number = 0
): RPCRequest => {
  return {
    jsonrpc: '2.0',
    id: 1,
    method: 'getBlock',
    params: [
      slot,
      {
        commitment,
        maxSupportedTransactionVersion
      }
    ]
  };
};

export const getTransactionPayload = (
  signature: string,
  commitment: Commitment['commitment'] = 'finalized',
  maxSupportedTransactionVersion: number = 0
): RPCRequest => {
  return {
    jsonrpc: '2.0',
    id: 1,
    method: 'getTransaction',
    params: [
      signature,
      {
        commitment,
        maxSupportedTransactionVersion
      }
    ]
  };
};

export const getSignaturesForAddressPayload = (
  address: string,
  before?: string,
  until?: string,
  limit: number = 1000,
  commitment: Commitment['commitment'] = 'finalized'
): RPCRequest => {
  const config: Record<string, any> = {
    limit
  };

  // Note: This endpoint doesn't support commitment parameter
  // if (commitment) config.commitment = commitment;
  if (before) config.before = before;
  if (until) config.until = until;

  return {
    jsonrpc: '2.0',
    id: 1,
    method: 'getSignaturesForAddress',
    params: [
      address,
      config
    ]
  };
};

export const getBlockTimePayload = (
  block: number,
  commitment: Commitment['commitment'] = 'finalized',
  maxSupportedTransactionVersion: number = 0
): RPCRequest => {
  return {
    jsonrpc: '2.0',
    id: 1,
    method: 'getBlockTime',
    params: [
      block,
      {
        commitment,
        maxSupportedTransactionVersion
      }
    ]
  };
};

export const getGenesisHashPayload = (): RPCRequest => {
  return {
    jsonrpc: '2.0',
    id: 1,
    method: 'getGenesisHash',
    params: []
  };
};

export const getFirstAvailableBlockPayload = (): RPCRequest => {
  return {
    jsonrpc: '2.0',
    id: 1,
    method: 'getFirstAvailableBlock',
    params: []
  };
};

export const getSlotPayload = (
  slot: number,
  commitment: Commitment['commitment'] = 'finalized',
  minContextSlot: number = 0,
): RPCRequest => {
  return {
    jsonrpc: '2.0',
    id: 1,
    method: 'getSlot',
    params: [
      {
        commitment,
        minContextSlot
      }
    ]
  };
};

export const getVersionPayload = (): RPCRequest => {
  return {
    jsonrpc: '2.0',
    id: 1,
    method: 'getVersion',
    params: []
  };
};


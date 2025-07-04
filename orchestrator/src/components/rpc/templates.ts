import { GRPCCategory, RPCCategory } from "./types"

export const RPC_TEMPLATES: Record<string, RPCCategory> = {
  "blocks": {
    name: "Block Methods",
    methods: {
      "getBlock": {
        name: "Get Block",
        description: "Returns information about a specific block by slot number",
        method: "getBlock",
        params: [
          { name: "slot", type: "number", required: true, description: "The slot number of the block" },
          { name: "encoding", type: "string", required: false, description: "Encoding format (json, jsonParsed, base58, base64)", default: "json" },
          { name: "transactionDetails", type: "string", required: false, description: "Level of transaction detail (full, accounts, signatures, none)", default: "full" },
          { name: "rewards", type: "boolean", required: false, description: "Whether to populate rewards array", default: false },
          { name: "commitment", type: "string", required: false, description: "Commitment level (finalized, confirmed, processed)", default: "finalized" }
        ],
        example: {
          jsonrpc: "2.0",
          method: "getBlock",
          params: [
            "{{slot}}",
            {
              encoding: "json",
              transactionDetails: "full",
              rewards: false
            }
          ],
          id: 1
        }
      },
      "getBlockTime": {
        name: "Get Block Time",
        description: "Returns the estimated production time of a block",
        method: "getBlockTime",
        params: [
          { name: "slot", type: "number", required: true, description: "The slot number of the block" }
        ],
        example: {
          jsonrpc: "2.0",
          method: "getBlockTime",
          params: ["{{slot}}"],
          id: 1
        }
      },
      "getFirstAvailableBlock": {
        name: "Get First Available Block",
        description: "Returns the slot of the lowest confirmed block that has not been purged",
        method: "getFirstAvailableBlock",
        params: [],
        example: {
          jsonrpc: "2.0",
          method: "getFirstAvailableBlock",
          params: [],
          id: 1
        }
      },
      "getSlot": {
        name: "Get Slot",
        description: "Returns the current slot the node is processing",
        method: "getSlot",
        params: [
          { name: "commitment", type: "string", required: false, description: "Commitment level (finalized, confirmed, processed)", default: "finalized" }
        ],
        example: {
          jsonrpc: "2.0",
          method: "getSlot",
          params: [
            {
              commitment: "finalized"
            }
          ],
          id: 1
        }
      }
    }
  },
  "transactions": {
    name: "Transaction Methods",
    methods: {
      "getTransaction": {
        name: "Get Transaction",
        description: "Returns transaction details for a confirmed transaction",
        method: "getTransaction",
        params: [
          { name: "signature", type: "string", required: true, description: "Transaction signature as base-58 encoded string" },
          { name: "encoding", type: "string", required: false, description: "Encoding format (json, jsonParsed, base58, base64)", default: "json" },
          { name: "commitment", type: "string", required: false, description: "Commitment level (finalized, confirmed)", default: "finalized" },
          { name: "maxSupportedTransactionVersion", type: "number", required: false, description: "Max transaction version to return", default: 0 }
        ],
        example: {
          jsonrpc: "2.0",
          method: "getTransaction",
          params: [
            "{{signature}}",
            {
              encoding: "json",
              commitment: "finalized"
            }
          ],
          id: 1
        }
      },
      "getSignaturesForAddress": {
        name: "Get Signatures for Address",
        description: "Returns signatures for confirmed transactions that include the given address",
        method: "getSignaturesForAddress",
        params: [
          { name: "address", type: "string", required: true, description: "Account address as base-58 encoded string" },
          { name: "limit", type: "number", required: false, description: "Maximum transaction signatures to return (1-1000)", default: 10 },
          { name: "before", type: "string", required: false, description: "Start searching backwards from this transaction signature" },
          { name: "until", type: "string", required: false, description: "Search until this transaction signature" },
          { name: "commitment", type: "string", required: false, description: "Commitment level (finalized, confirmed)", default: "finalized" }
        ],
        example: {
          jsonrpc: "2.0",
          method: "getSignaturesForAddress",
          params: [
            "{{address}}",
            {
              limit: 10,
              commitment: "finalized"
            }
          ],
          id: 1
        }
      }
    }
  },
  "network": {
    name: "Network & Info Methods",
    methods: {
      "getGenesisHash": {
        name: "Get Genesis Hash",
        description: "Returns the genesis hash (for epoch 0)",
        method: "getGenesisHash",
        params: [],
        example: {
          jsonrpc: "2.0",
          method: "getGenesisHash",
          params: [],
          id: 1
        }
      },
      "getVersion": {
        name: "Get Version",
        description: "Returns the current Solana version running on the node",
        method: "getVersion",
        params: [],
        example: {
          jsonrpc: "2.0",
          method: "getVersion",
          params: [],
          id: 1
        }
      }
    }
  }
}

export const GRPC_TEMPLATES: Record<string, GRPCCategory> = {
  "blocks": {
    name: "Block Methods (gRPC)",
    methods: {
      "GetBlock": {
        name: "Get Block",
        description: "Returns information about a specific block by slot number",
        method: "GetBlock",
        params: [
          { name: "slot", type: "number", required: true, description: "The slot number of the block" }
        ],
        example: `grpcurl \\
  -proto old-faithful.proto \\
  -H 'x-token: <token>' \\
  -d '{"slot": 307152000}' \\
  customer-endpoint-2608.mainnet.rpcpool.com:443 \\
  OldFaithful.OldFaithful/GetBlock`
      },
      "GetBlockTime": {
        name: "Get Block Time",
        description: "Returns the estimated production time of a block",
        method: "GetBlockTime",
        params: [
          { name: "slot", type: "number", required: true, description: "The slot number of the block" }
        ],
        example: `grpcurl \\
  -proto old-faithful.proto \\
  -H 'x-token: <token>' \\
  -d '{"slot": 307152000}' \\
  customer-endpoint-2608.mainnet.rpcpool.com:443 \\
  OldFaithful.OldFaithful/GetBlockTime`
      },
      "GetFirstAvailableBlock": {
        name: "Get First Available Block",
        description: "Returns the slot of the lowest confirmed block that has not been purged",
        method: "GetFirstAvailableBlock",
        params: [],
        example: `grpcurl \\
  -proto old-faithful.proto \\
  -H 'x-token: <token>' \\
  -d '' \\
  customer-endpoint-2608.mainnet.rpcpool.com:443 \\
  OldFaithful.OldFaithful/GetFirstAvailableBlock`
      },
      "GetSlot": {
        name: "Get Slot",
        description: "Returns the current slot the node is processing",
        method: "GetSlot",
        params: [],
        example: `grpcurl \\
  -proto old-faithful.proto \\
  -H 'x-token: <token>' \\
  -d '' \\
  customer-endpoint-2608.mainnet.rpcpool.com:443 \\
  OldFaithful.OldFaithful/GetSlot`
      },
      "StreamBlocks": {
        name: "Stream Blocks",
        description: "Stream blocks within a slot range with optional filters",
        method: "StreamBlocks",
        streaming: true,
        params: [
          { name: "start_slot", type: "number", required: true, description: "Starting slot number" },
          { name: "end_slot", type: "number", required: true, description: "Ending slot number" },
          { name: "filter", type: "object", required: false, description: "Optional filter for account includes" }
        ],
        example: `grpcurl \\
  -proto old-faithful.proto \\
  -H 'x-token: <token>' \\
  -d '{"start_slot": 307152000, "end_slot": 307152010}' \\
  customer-endpoint-2608.mainnet.rpcpool.com:443 \\
  OldFaithful.OldFaithful/StreamBlocks`
      }
    }
  },
  "transactions": {
    name: "Transaction Methods (gRPC)",
    methods: {
      "GetTransaction": {
        name: "Get Transaction",
        description: "Returns transaction details for a confirmed transaction",
        method: "GetTransaction",
        params: [
          { name: "signature", type: "string", required: true, description: "Transaction signature as base-64 encoded string" }
        ],
        example: `grpcurl \\
  -proto old-faithful.proto \\
  -H 'x-token: <token>' \\
  -d '{"signature": "GbXoI+D7hhgeiUwovUhtaxog6zsxFcd5PKfhQM85GR6+NqmiFmQDf9cCCVj8BRj+DR1RvgR/E2E/ckbSGuQKCg=="}' \\
  customer-endpoint-2608.mainnet.rpcpool.com:443 \\
  OldFaithful.OldFaithful/GetTransaction`
      },
      "StreamTransactions": {
        name: "Stream Transactions",
        description: "Stream transactions within a slot range with optional filters",
        method: "StreamTransactions",
        streaming: true,
        params: [
          { name: "start_slot", type: "number", required: true, description: "Starting slot number" },
          { name: "end_slot", type: "number", required: true, description: "Ending slot number" },
          { name: "filter", type: "object", required: false, description: "Optional filter for vote/failed transactions" }
        ],
        example: `grpcurl \\
  -proto old-faithful.proto \\
  -H 'x-token: <token>' \\
  -d '{"start_slot": 307152000, "end_slot": 307152010, "filter": {"vote": false, "failed": true}}' \\
  customer-endpoint-2608.mainnet.rpcpool.com:443 \\
  OldFaithful.OldFaithful/StreamTransactions`
      }
    }
  },
  "network": {
    name: "Network & Info Methods (gRPC)",
    methods: {
      "GetGenesisHash": {
        name: "Get Genesis Hash",
        description: "Returns the genesis hash (for epoch 0)",
        method: "GetGenesisHash",
        params: [],
        example: `grpcurl \\
  -proto old-faithful.proto \\
  -H 'x-token: <token>' \\
  -d '' \\
  customer-endpoint-2608.mainnet.rpcpool.com:443 \\
  OldFaithful.OldFaithful/GetGenesisHash`
      },
      "GetVersion": {
        name: "Get Version",
        description: "Returns the current Old Faithful version",
        method: "GetVersion",
        params: [],
        example: `grpcurl \\
  -proto old-faithful.proto \\
  -H 'x-token: <token>' \\
  -d '' \\
  customer-endpoint-2608.mainnet.rpcpool.com:443 \\
  OldFaithful.OldFaithful/GetVersion`
      }
    }
  }
} 
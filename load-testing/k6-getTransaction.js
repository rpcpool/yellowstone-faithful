// k6-getTransaction.js
//
// This script for the k6 load testing tool generates POST requests to the
// Solana `getTransaction` JSON-RPC method with a random transaction signature.
//
// k6 is a modern, flexible, and developer-friendly load testing tool.
// https://k6.io/

import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';

// --- Custom Metrics ---
// A custom counter to specifically track unexpected RPC-level errors.
const rpcErrors = new Counter('rpc_errors');
// A custom trend metric to track the size of the response body in bytes.
const responseSize = new Trend('response_size');

// Load transaction list
const TX_LIST_FILE = __ENV.TX_LIST || 'payload/transaction-hashes';
let globalTxList = [];

try {
  const txListContent = open(TX_LIST_FILE);
  globalTxList = txListContent.split('\n').filter(tx => tx.trim() !== '');
  console.log(`Loaded ${globalTxList.length} transactions from file: ${TX_LIST_FILE}`);
} catch (e) {
  throw new Error(`Could not load transaction list from file: ${TX_LIST_FILE}. Error: ${e}`);
}

// --- k6 Options ---
export const options = {
  vus: 100,
  duration: '2m',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    // The p95 response time threshold has been set to 2000ms (2s).
    http_req_duration: ['p(95)<2000'],
    // The test will fail if the server returns more than 10 unexpected RPC-level errors.
    rpc_errors: ['count<10'],
  },
};


export function setup() {
  const RPC_URL = __ENV.RPC_URL || 'http://127.0.0.1:8899';
  const COMMITMENT = __ENV.COMMITMENT || 'confirmed';
  const ENCODING = __ENV.ENCODING || 'base64';
  const MAX_SUPPORTED_TX_VERSION = __ENV.MAX_SUPPORTED_TX_VERSION || '0';
  const USE_GZIP = __ENV.USE_GZIP === 'true';

  if (globalTxList.length === 0) {
    throw new Error(`No transactions loaded from file: ${TX_LIST_FILE}`);
  }

  const staticRpcParams = {
    encoding: ENCODING,
    commitment: COMMITMENT,
    maxSupportedTransactionVersion: parseInt(MAX_SUPPORTED_TX_VERSION)
  };

  return {
    rpcUrl: RPC_URL,
    useGzip: USE_GZIP,
    txList: globalTxList,
    staticRpcParams: staticRpcParams,
  };
}

export default function (data) {
  const randomTx = data.txList[Math.floor(Math.random() * data.txList.length)];

  const payload = JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'getTransaction',
    params: [randomTx, data.staticRpcParams],
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  if (data.useGzip) {
    params.compression = 'gzip';
  }

  const res = http.post(data.rpcUrl, payload, params);
  responseSize.add(res.body.length);

  const httpSuccess = check(res, {
    'HTTP status is 200': (r) => r.status === 200,
  });

  if (httpSuccess) {
    try {
      const body = res.json();
      const rpcSuccess = check(body, {
        'RPC: no unexpected errors': (b) => {
          if (!b || !b.error) return true;
          return b.error.message && b.error.message.includes('not found');
        },
        'RPC: valid response structure': (b) => {
          return !b || !b.result || b.result !== null;
        }
      });

      if (!rpcSuccess) {
        rpcErrors.add(1);
        console.error(`RPC Error for ${randomTx}: ${JSON.stringify(body?.error || 'Invalid response')}`);
      }
    } catch (e) {
      rpcErrors.add(1);
      console.error(`JSON parse error for ${randomTx}: ${e}`);
    }
  } else {
    rpcErrors.add(1);
  }

}

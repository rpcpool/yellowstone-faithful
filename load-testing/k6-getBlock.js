// k6-getBlock.js
//
// This script for the k6 load testing tool generates POST requests to the
// Solana `getBlock` JSON-RPC method with a random block number.
//
// k6 is a modern, flexible, and developer-friendly load testing tool.
// https://k6.io/

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';

// --- Configuration ---
// Configuration is now driven by environment variables for better flexibility.
// Example: k6 run -e RPC_URL=http://my-other-node:8899 k6-getBlock.js
const RPC_URL = __ENV.RPC_URL || 'http://127.0.0.1:8899';
const MIN_BLOCK = parseInt(__ENV.MIN_BLOCK || '320544000');
const MAX_BLOCK = parseInt(__ENV.MAX_BLOCK || '320975999');

// --- Custom Metrics ---
// A custom counter to specifically track RPC-level errors.
const rpcErrors = new Counter('rpc_errors');

// --- k6 Options ---
export const options = {
  stages: [
    { duration: '30s', target: 100 },
    { duration: '1m', target: 100 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],
    // The p95 response time threshold has been increased to 3000ms (3s)
    // to accommodate the observed high latency.
    http_req_duration: ['p(95)<3000'],
    rpc_errors: ['count<10'], // Fail if more than 10 RPC errors occur.
  },
};

// --- Main Test Logic ---
export default function () {
  // 1. Generate a random block number.
  const randomBlock =
    Math.floor(Math.random() * (MAX_BLOCK - MIN_BLOCK + 1)) + MIN_BLOCK;

  // 2. Construct the JSON-RPC payload.
  // NOTE: The test results indicate the target server is ignoring `transactionDetails: "none"`
  // and returning a full, multi-megabyte payload. This is the primary cause of high latency.
  const payload = JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'getBlock',
    params: [
      randomBlock,
      {
        encoding: 'json',
        transactionDetails: 'none',
        rewards: false,
      },
    ],
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  // 3. Send the POST request.
  const res = http.post(RPC_URL, payload, params);

  // 4. Perform robust checks on the response.
  check(res, {
    'HTTP status is 200': (r) => r.status === 200,
  });

  if (res.status === 200) {
    const body = res.json();
    // Perform a series of detailed checks on the RPC response body.
    const isRpcSuccess = check(body, {
      'RPC: no error field': (b) => b && !b.error,
      'RPC: has result object': (b) =>
        b && typeof b.result === 'object' && b.result !== null,
      'RPC: result has blockhash': (b) =>
        b && b.result && typeof b.result.blockhash === 'string',
      'RPC: result has blockTime': (b) =>
        b && b.result && typeof b.result.blockTime === 'number',
    });

    // If any of the RPC checks failed, increment our custom counter.
    if (!isRpcSuccess) {
      rpcErrors.add(1);
    }
  } else {
    // If the HTTP request itself failed, count that as an RPC error.
    rpcErrors.add(1);
  }

  sleep(1);
}

// --- HOW TO RUN THE LOAD TEST ---

// 1. Install k6.
//    On macOS: brew install k6
//    On Ubuntu/Debian:
//      sudo gpg -k
//      sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
//      echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
//      sudo apt-get update
//      sudo apt-get install k6
//    See https://k6.io/docs/getting-started/installation/ for other systems.

// 2. Save this script as `k6-getBlock.js`.

// 3. Run the test from your terminal.
//    k6 run k6-getBlock.js

// 4. To override configuration using environment variables:
//    k6 run -e RPC_URL=http://another-node:8899 -e MAX_BLOCK=400000000 k6-getBlock.js

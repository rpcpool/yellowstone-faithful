// k6-getBlock.js
//
// This script for the k6 load testing tool generates POST requests to the
// Solana `getBlock` JSON-RPC method with a random block number.
//
// k6 is a modern, flexible, and developer-friendly load testing tool.
// https://k6.io/

import http from 'k6/http';
import { check, sleep } from 'k6';

// --- Configuration ---

// The Solana JSON-RPC endpoint to target.
const RPC_URL = 'http://127.0.0.1:8899';

// The range of block numbers to request.
const MIN_BLOCK = 320544000;
const MAX_BLOCK = 320975999;

// --- k6 Options ---
// This section configures the load profile for the test.
// See https://k6.io/docs/using-k6/options/
export const options = {
  // Define stages for a ramp-up and sustained load pattern.
  stages: [
    { duration: '30s', target: 100 }, // Ramp up to 100 virtual users over 30 seconds
    { duration: '1m', target: 100 }, // Stay at 100 virtual users for 1 minute
    { duration: '10s', target: 0 }, // Ramp down to 0 users
  ],
  // Define thresholds for success criteria. The test will fail if these are not met.
  thresholds: {
    http_req_failed: ['rate<0.01'], // Fail if HTTP error rate is > 1%
    http_req_duration: ['p(95)<500'], // Fail if 95th percentile response time is > 500ms
  },
};

// --- Main Test Logic ---
// This is the default function that k6 executes for each "virtual user" (VU).
// A VU will run this function in a loop for the duration of the test.
export default function () {
  // 1. Generate a random block number for this iteration using standard JavaScript.
  // This is compatible with all versions of k6.
  const randomBlock = Math.floor(
    Math.random() * (MAX_BLOCK - MIN_BLOCK + 1) + MIN_BLOCK,
  );

  // 2. Construct the JSON-RPC payload.
  // We set `transactionDetails` to `none` to keep the response size small.
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

  // 3. Define the headers for the POST request.
  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  // 4. Send the POST request.
  const res = http.post(RPC_URL, payload, params);

  // 5. Check the response to verify it was successful.
  check(res, {
    'status is 200': (r) => r.status === 200,
    'body contains result': (r) => r.body.includes('result'),
  });

  // Add a short sleep to pace the requests slightly.
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

// 2. Cd into the directory where `k6-getBlock.js` is located.

// 3. Run the test from your terminal.
//    The load profile (users, duration) is defined in the `options` section of the script.
//
//    k6 run k6-getBlock.js

// 4. To override options from the command line:
//
//    k6 run --vus 10 --duration 30s k6-getBlock.js

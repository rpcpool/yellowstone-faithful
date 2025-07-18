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

// --- Custom Metrics ---
// A custom counter to specifically track unexpected RPC-level errors.
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
    http_req_duration: ['p(95)<2000'],
    // The test will fail if the server returns more than 10 unexpected RPC-level errors.
    rpc_errors: ['count<10'],
  },
};

// --- Setup Function ---
// This function runs once before the test starts, initializing configuration
// and passing it to the virtual users. This prevents the setup logic from
// running for every VU.
export function setup() {
  const RPC_URL = __ENV.RPC_URL || 'http://127.0.0.1:8899';
  let EPOCHS = __ENV.EPOCHS; // Expects a comma-separated list, e.g., "742,745,750"
  const EPOCH_LEN = 432000;
  const USE_GZIP = __ENV.USE_GZIP === 'true';

  let blockRanges = [];

  // If EPOCHS environment variable is not provided, try fetching from the API.
  if (!EPOCHS) {
    console.log(
      'EPOCHS environment variable not set. Attempting to fetch from API...',
    );
    const epochsApiUrl = `${RPC_URL}/api/v1/epochs`;
    const res = http.get(epochsApiUrl);

    if (res.status === 200) {
      try {
        const responseData = res.json();
        const fetchedEpochs = responseData.epochs; // Access the nested 'epochs' array.
        if (Array.isArray(fetchedEpochs) && fetchedEpochs.length > 0) {
          console.log(
            `Successfully fetched ${fetchedEpochs.length} epochs from API.`,
          );
          EPOCHS = fetchedEpochs.join(','); // Convert array to comma-separated string to reuse parsing logic
        } else {
          console.log(
            'API returned no epochs or an invalid format. Falling back to default block range.',
          );
        }
      } catch (e) {
        console.log(
          `Failed to parse JSON from epochs API: ${e}. Falling back to default block range.`,
        );
      }
    } else {
      console.log(
        `Failed to fetch epochs from API (status: ${res.status}). Falling back to default block range.`,
      );
    }
  }

  // If a list of epochs is available (from env var or API), calculate the block ranges.
  if (EPOCHS) {
    const epochList = EPOCHS.split(',');
    console.log(`Using epochs: ${EPOCHS}. Calculating block ranges...`);
    epochList.forEach((epochStr) => {
      const epoch = parseInt(epochStr.trim());
      if (!isNaN(epoch)) {
        const minBlock = epoch * EPOCH_LEN;
        const maxBlock = minBlock + EPOCH_LEN - 1;
        blockRanges.push({ min: minBlock, max: maxBlock });
        console.log(
          `  - Added range for epoch ${epoch}: ${minBlock} to ${maxBlock}`,
        );
      }
    });
  }

  // If, after all attempts, blockRanges is still empty, use the hardcoded default.
  if (blockRanges.length === 0) {
    const minBlock = parseInt(__ENV.MIN_BLOCK || '320544000');
    const maxBlock = parseInt(__ENV.MAX_BLOCK || '320975999');
    blockRanges.push({ min: minBlock, max: maxBlock });
    console.log(
      `Using hardcoded default block range: ${minBlock} to ${maxBlock}`,
    );
  }

  // Return the configuration so it can be used in the VU code.
  return {
    rpcUrl: RPC_URL,
    blockRanges: blockRanges,
    useGzip: USE_GZIP,
  };
}

// --- Main Test Logic ---
// The `data` parameter receives the object returned from the setup() function.
export default function (data) {
  // 1. Select a random epoch range from the configured list.
  const selectedRange =
    data.blockRanges[Math.floor(Math.random() * data.blockRanges.length)];

  // 2. Generate a random block number within that selected range.
  const randomBlock =
    Math.floor(Math.random() * (selectedRange.max - selectedRange.min + 1)) +
    selectedRange.min;

  // 3. Construct the JSON-RPC payload.
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

  // Add gzip compression option if enabled via environment variable.
  if (data.useGzip) {
    params.compression = 'gzip';
  }

  // 4. Send the POST request.
  const res = http.post(data.rpcUrl, payload, params);

  // 5. Perform robust checks on the response.
  const httpSuccess = check(res, {
    'HTTP status is 200': (r) => r.status === 200,
  });

  if (httpSuccess) {
    try {
      const body = res.json();
      // Perform a series of detailed checks on the RPC response body.
      const rpcSuccess = check(body, {
        'RPC: no error field OR is a known acceptable error': (b) => {
          if (!b || !b.error) {
            return true; // No error field, which is a success.
          }
          // Check if the error is the acceptable "skipped slot" error.
          if (
            b.error.message &&
            b.error.message.includes(
              'was skipped, or missing in long-term storage',
            )
          ) {
            return true; // This is an acceptable error, so the check passes.
          }
          return false; // An unexpected error occurred.
        },
      });

      // If the RPC check failed, it means we encountered an *unexpected* error.
      if (!rpcSuccess) {
        rpcErrors.add(1);
        // Log the specific unexpected error for debugging.
        if (body && body.error) {
          console.error(
            `Unexpected RPC Error on block ${randomBlock}: ${JSON.stringify(
              body.error,
            )}`,
          );
        } else {
          console.error(
            `Malformed RPC response on block ${randomBlock}: ${res.body}`,
          );
        }
      }
    } catch (e) {
      // This block catches errors from res.json() if the body is not valid JSON.
      rpcErrors.add(1);
      console.error(
        `Failed to parse JSON response for block ${randomBlock}. Error: ${e}. Body: ${res.body}`,
      );
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

// 3. Run the test from your terminal. It will attempt to fetch epochs from the API.
//    k6 run k6-getBlock.js

// 4. To run with a specific list of epochs, overriding the API call:
//    k6 run -e EPOCHS="742,745,750" k6-getBlock.js

// 5. To run with gzip compression enabled:
//    k6 run -e USE_GZIP=true k6-getBlock.js

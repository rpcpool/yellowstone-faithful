// k6-getBlock.js
//
// This script for the k6 load testing tool generates POST requests to the
// Solana `getBlock` JSON-RPC method with a random block number.
//
// k6 is a modern, flexible, and developer-friendly load testing tool.
// https://k6.io/

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.1.0/index.js';

// --- Custom Metrics ---
// A custom counter to specifically track unexpected RPC-level errors.
const rpcErrors = new Counter('rpc_errors');
// A custom trend metric to track the size of the response body in bytes.
const responseSize = new Trend('response_size');

// --- k6 Options ---
export const options = {
  stages: [
    { duration: '30s', target: 100 },
    { duration: '1m', target: 100 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],
    // The p95 response time threshold has been lowered to 2000ms (2s).
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

  // Add the response size to our custom trend metric.
  responseSize.add(res.body.length);

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

// --- Summary Function ---
// This function runs at the end of the test and generates the final report.
export function handleSummary(data) {
  // Helper function to format bytes into a human-readable string.
  function formatBytes(bytes, decimals = 2) {
    if (!+bytes) return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
  }

  let customReport = '';

  // Use optional chaining (?.) to safely access the nested 'values' property.
  // This is the most robust way to prevent crashes if the metric data is inconsistent.
  const responseStats = data.metrics.response_size?.values;

  // Check if we successfully retrieved the stats object.
  if (responseStats) {
    // Build a custom summary string.
    customReport += '\n\n█ HUMAN-READABLE RESPONSE SIZE\n\n';
    customReport += `  response_size.......................................................: avg=${formatBytes(
      responseStats.avg,
    )} min=${formatBytes(responseStats.min)} med=${formatBytes(
      responseStats.med,
    )} max=${formatBytes(responseStats.max)} p(90)=${formatBytes(
      responseStats['p(90)'],
    )} p(95)=${formatBytes(responseStats['p(95)'])}`;
  } else {
    customReport +=
      '\n\n█ HUMAN-READABLE RESPONSE SIZE\n\n  (No response size data was collected, likely due to all requests failing.)';
  }

  // Return the standard text summary, appending our custom report to it.
  return {
    stdout:
      textSummary(data, { indent: ' ', enableColors: true }) + customReport,
    'summary.json': JSON.stringify(data), // Optional: produce a machine-readable JSON summary
  };
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

// --- Correlation Analysis ---
// To correlate response size and latency, you must export the raw results to a file
// and analyze it with an external tool (e.g., Python with pandas/matplotlib, R, etc.).
//
// 5. Run the test and output to a JSON file:
//    k6 run --out json=results.json k6-getBlock.js
//
// 6. You can then parse `results.json`. Each line is a JSON object. Look for objects
//    where `type` is "Point" and `metric` is `http_req_duration` or `response_size`.
//    You can then match these points by their timestamp (`data.time`) to correlate them.

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
import exec from 'k6/execution';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.6.0/index.js';

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

// This global variable will hold the configuration from the setup function
// so that it can be accessed in the handleSummary function.
let setupConfig = {};

// --- PRNG for Deterministic Runs ---
// A simple Linear Congruential Generator for seeded random numbers.
function PRNG(seed) {
  this.m = 0x80000000; // 2**31
  this.a = 1103515245;
  this.c = 12345;
  this.state = seed ? seed : Math.floor(Math.random() * (this.m - 1));

  this.nextInt = function () {
    this.state = (this.a * this.state + this.c) % this.m;
    return this.state;
  };

  this.random = function () {
    // returns in range [0,1]
    return this.nextInt() / (this.m - 1);
  };
}

// --- Setup Function ---
// This function runs once before the test starts, initializing configuration
// and passing it to the virtual users.
export function setup() {
  const RPC_URL = __ENV.RPC_URL || 'http://127.0.0.1:8899';
  let EPOCHS_RAW = __ENV.EPOCHS; // Expects a comma-separated list, e.g., "742,745,750"
  let EPOCHS_PARSED = EPOCHS_RAW;
  const EPOCH_LEN = 432000;
  const USE_GZIP = __ENV.USE_GZIP === 'true';
  const SEED = __ENV.SEED ? parseInt(__ENV.SEED) : null;

  // The initial PRNG is only used to generate a seed if one isn't provided.
  const initialPrng = new PRNG(SEED);
  const usedSeed = initialPrng.state;
  if (SEED) {
    console.log(`Using user-provided SEED for PRNG: ${usedSeed}`);
  } else {
    console.log(
      `No SEED provided. Generated a new random seed for PRNG: ${usedSeed}. Use this seed to reproduce the run.`,
    );
  }

  let blockRanges = [];
  let epochSource = 'default';

  // If EPOCHS environment variable is not provided, try fetching from the API.
  if (!EPOCHS_PARSED) {
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
          epochSource = `API (${epochsApiUrl})`;
          console.log(
            `Successfully fetched ${fetchedEpochs.length} epochs from API.`,
          );
          EPOCHS_PARSED = fetchedEpochs.join(','); // Convert array to comma-separated string to reuse parsing logic
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
  } else {
    epochSource = 'Environment Variable (EPOCHS)';
  }

  // If a list of epochs is available (from env var or API), calculate the block ranges.
  if (EPOCHS_PARSED) {
    const epochList = EPOCHS_PARSED.split(',');
    console.log(`Using epochs: ${EPOCHS_PARSED}. Calculating block ranges...`);
    epochList.forEach((epochStr) => {
      const epoch = parseInt(epochStr.trim());
      if (!isNaN(epoch)) {
        const minBlock = epoch * EPOCH_LEN;
        const maxBlock = minBlock + EPOCH_LEN - 1;
        blockRanges.push({ epoch: epoch, min: minBlock, max: maxBlock });
        console.log(
          `  - Added range for epoch ${epoch}: ${minBlock} to ${maxBlock}`,
        );
      }
    });
  }

  // If, after all attempts, blockRanges is still empty, use the hardcoded default.
  if (blockRanges.length === 0) {
    epochSource = 'Hardcoded Default';
    const minBlock = parseInt(__ENV.MIN_BLOCK || '320544000');
    const maxBlock = parseInt(__ENV.MAX_BLOCK || '320975999');
    blockRanges.push({ epoch: null, min: minBlock, max: maxBlock });
    console.log(
      `Using hardcoded default block range: ${minBlock} to ${maxBlock}`,
    );
  }

  const staticRpcParams = {
    encoding: 'json',
    transactionDetails: 'none',
    rewards: false,
  };

  // Store the final configuration in the global variable for handleSummary.
  setupConfig = {
    runID: uuidv4(),
    k6Version: exec.version,
    seed: usedSeed,
    rpcUrl: RPC_URL,
    useGzip: USE_GZIP,
    epochSource: epochSource,
    epochsRaw: EPOCHS_RAW || null, // Original env var value
    epochsUsed: EPOCHS_PARSED, // The list that was actually used
    blockRanges: blockRanges,
    staticRpcParams: staticRpcParams,
  };

  // Return the configuration so it can be used in the VU code.
  return setupConfig;
}

// --- Main Test Logic ---
// The `data` parameter receives the object returned from the setup() function.
export default function (data) {
  // 1. Create a deterministic PRNG for this specific iteration.
  // The seed is a combination of the main run seed, the VU's unique ID, and the VU's iteration number.
  // This ensures that VU #5 on its 10th iteration will ALWAYS request the same block,
  // guaranteeing a reproducible sequence of requests regardless of execution speed.
  const perIterationSeed =
    data.seed + exec.vu.idInTest * 1000000 + exec.vu.iterationInScenario;
  const prng = new PRNG(perIterationSeed);

  // 2. Select a random epoch range from the configured list using the deterministic PRNG.
  const selectedRange =
    data.blockRanges[Math.floor(prng.random() * data.blockRanges.length)];

  // 3. Generate a random block number within that selected range using the deterministic PRNG.
  const randomBlock =
    Math.floor(prng.random() * (selectedRange.max - selectedRange.min + 1)) +
    selectedRange.min;

  // 4. Construct the JSON-RPC payload.
  const payload = JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'getBlock',
    params: [randomBlock, data.staticRpcParams],
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  // Add gzip compression option if enabled via environment variable.
  if (data.useGzip) {
    params.compression = 'gzip';
  }

  // 5. Send the POST request.
  const res = http.post(data.rpcUrl, payload, params);

  // Add the response size to our custom trend metric.
  responseSize.add(res.body.length);

  // 6. Perform robust checks on the response.
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
  // This function manually recreates the structure of the default k6 summary report,
  // including distinct sections for thresholds and results, while also adding the
  // custom human-readable response size report.

  function formatBytes(bytes, decimals = 2) {
    if (
      bytes === null ||
      bytes === undefined ||
      !isFinite(bytes) ||
      bytes === 0
    )
      return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
  }

  function formatDuration(ms) {
    if (ms < 1000) return `${ms.toFixed(2)}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  }

  const green = (text) => `\x1b[32m${text}\x1b[0m`;
  const red = (text) => `\x1b[31m${text}\x1b[0m`;

  let summary = ['\n\n█ THRESHOLDS\n'];

  for (const t of Object.keys(data.thresholds)) {
    const metricName = t.split('{')[0];
    const metric = data.metrics[metricName];
    if (metric && metric.thresholds && metric.thresholds[t]) {
      const threshold = metric.thresholds[t];
      const pass = threshold.ok;
      const symbol = pass ? green('✓') : red('✗');

      let valueStr = '';
      if (metric.type === 'trend') {
        const pValue = t.match(/p\((\d+\.?\d*)\)/);
        if (pValue) {
          valueStr = `p(${pValue[1]})=${formatDuration(
            metric.values[pValue[0]],
          )}`;
        }
      } else if (metric.type === 'rate') {
        valueStr = `rate=${(metric.values.rate * 100).toFixed(2)}%`;
      } else if (metric.type === 'counter') {
        valueStr = `count=${metric.values.count}`;
      }

      summary.push(`\n  ${metricName}`);
      summary.push(`    ${symbol} '${t}' ${valueStr}`);
    }
  }

  summary.push('\n\n█ TOTAL RESULTS\n');

  const checks = data.metrics.checks;
  summary.push(
    `\n  checks.........................: ${(
      (checks.values.passes / (checks.values.passes + checks.values.fails)) *
      100
    ).toFixed(2)}%   ${green('✓ ' + checks.values.passes)}   ${red(
      '✗ ' + checks.values.fails,
    )}`,
  );

  for (const [name, metric] of Object.entries(data.metrics)) {
    if (name === 'checks') continue; // Already handled
    let line = `\n  ${name}......................:`;
    if (metric.type === 'trend') {
      line += ` avg=${formatDuration(metric.values.avg)} min=${formatDuration(
        metric.values.min,
      )} med=${formatDuration(metric.values.med)} max=${formatDuration(
        metric.values.max,
      )} p(90)=${formatDuration(metric.values['p(90)'])} p(95)=${formatDuration(
        metric.values['p(95)'],
      )}`;
    } else if (metric.type === 'counter') {
      line += ` ${metric.values.count}   ${metric.values.rate.toFixed(2)}/s`;
    } else if (metric.type === 'gauge') {
      line += ` value=${metric.values.value} min=${metric.values.min} max=${metric.values.max}`;
    }
    summary.push(line);
  }

  const responseStats = data.metrics.response_size?.values;
  if (responseStats) {
    summary.push('\n\n█ HUMAN-READABLE RESPONSE SIZE\n');
    summary.push(
      `\n  response_size.......................................................: avg=${formatBytes(
        responseStats.avg,
      )} min=${formatBytes(responseStats.min)} med=${formatBytes(
        responseStats.med,
      )} max=${formatBytes(responseStats.max)} p(90)=${formatBytes(
        responseStats['p(90)'],
      )} p(95)=${formatBytes(responseStats['p(95)'])}`,
    );
  }

  const fullSummary = {
    configuration: setupConfig,
    results: data,
  };

  const timestamp = new Date()
    .toISOString()
    .replace(/:/g, '-')
    .replace(/\..+/, '');
  const jsonFilename = `summary-${setupConfig.runID}-${timestamp}.json`;

  return {
    stdout: summary.join(''),
    [jsonFilename]: JSON.stringify(fullSummary, null, 2),
  };
}

// --- HOW TO RUN THE LOAD TEST ---

// 1. Install k6.
//    On macOS: brew install k6
//    On Ubuntu/Debian:
//      sudo gpg -k
//      sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C4P91D6AC1D69
//      echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
//      sudo apt-get update
//      sudo apt-get install k6
//    See https://k6.io/docs/getting-started/installation/ for other systems.

// 2. Save this script as `k6-getBlock.js`.

// 3. Run the test from your terminal. It will attempt to fetch epochs from the API.
//    k6 run k6-getBlock.js

// 4. To run with a specific list of epochs, overriding the API call:
//    k6 run -e EPOCHS="742,745,750" k6-getBlock.js

// 5. To reproduce a specific run, use the SEED from the logs:
//    k6 run -e SEED=123456789 k6-getBlock.js

// --- Correlation Analysis ---
// To correlate response size and latency, you must export the raw results to a file
// and analyze it with an external tool (e.g., Python with pandas/matplotlib, R, etc.).
//
// 6. Run the test and output to a JSON file:
//    k6 run --out json=results.json k6-getBlock.js
//
// 7. You can then parse `results.json`. Each line is a JSON object. Look for objects
//    where `type` is "Point" and `metric` is `http_req_duration` or `response_size`.
//    You can then match these points by their timestamp (`data.time`) to correlate them.

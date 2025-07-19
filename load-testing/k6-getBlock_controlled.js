// k6-getBlock.js
//
// This script for the k6 load testing tool generates POST requests to the
// Solana `getBlock` JSON-RPC method with a random block number.
// This version is designed to be used with the `externally-controlled` executor,
// allowing a separate script to dynamically control the number of VUs during the test.
//
// k6 is a modern, flexible, and developer-friendly load testing tool.
// https://k6.io/

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import exec from 'k6/execution';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.6.0/index.js';

// --- Custom Metrics ---
const rpcErrors = new Counter('rpc_errors');
const responseSize = new Trend('response_size');
// A dedicated Trend metric for the latency-based back-off logic.
const vu_latency_trend = new Trend('vu_latency', true);

const LATENCY_THRESHOLD_MS = 500;

// --- k6 Options ---
export const options = {
  // Define the scenario with the externally-controlled executor
  scenarios: {
    'externally-controlled-scenario': {
      executor: 'externally-controlled',
      // Define the total duration of the test. The controller will manage this.
      duration: '1h',
      // Define the maximum possible VUs. The controller cannot exceed this.
      maxVUs: 1000,
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: [`p(95)<${LATENCY_THRESHOLD_MS}`],
    rpc_errors: ['count<10'],
    // Threshold for our custom latency metric.
    vu_latency: [`p(95)<${LATENCY_THRESHOLD_MS}`],
  },
};

// --- PRNG for Deterministic Runs ---
function PRNG(seed) {
  this.m = 0x80000000;
  this.a = 1103515245;
  this.c = 12345;
  this.state = seed ? seed : Math.floor(Math.random() * (this.m - 1));
  this.nextInt = function () {
    this.state = (this.a * this.state + this.c) % this.m;
    return this.state;
  };
  this.random = function () {
    return this.nextInt() / (this.m - 1);
  };
}

// --- Setup Function ---
export function setup() {
  console.log(
    'Running in externally-controlled mode. A separate controller script is required to manage VUs.',
  );
  const RPC_URL = __ENV.RPC_URL || 'http://127.0.0.1:8899';
  let EPOCHS_RAW = __ENV.EPOCHS;
  let EPOCHS_PARSED = EPOCHS_RAW;
  const EPOCH_LEN = 432000;
  const USE_GZIP = __ENV.USE_GZIP === 'true';
  const SEED = __ENV.SEED ? parseInt(__ENV.SEED) : null;
  const TRANSACTION_DETAILS = __ENV.TRANSACTION_DETAILS || 'none';
  const REWARDS = __ENV.REWARDS === 'true';
  const ENCODING = __ENV.ENCODING || 'json';

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

  if (!EPOCHS_PARSED) {
    console.log(
      'EPOCHS environment variable not set. Attempting to fetch from API...',
    );
    const epochsApiUrl = `${RPC_URL}/api/v1/epochs`;
    const res = http.get(epochsApiUrl);
    if (res.status === 200) {
      try {
        const responseData = res.json();
        const fetchedEpochs = responseData.epochs;
        if (Array.isArray(fetchedEpochs) && fetchedEpochs.length > 0) {
          epochSource = `API (${epochsApiUrl})`;
          console.log(
            `Successfully fetched ${fetchedEpochs.length} epochs from API.`,
          );
          EPOCHS_PARSED = fetchedEpochs.join(',');
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
    encoding: ENCODING,
    transactionDetails: TRANSACTION_DETAILS,
    rewards: REWARDS,
  };
  console.log(`Using RPC parameters: ${JSON.stringify(staticRpcParams)}`);

  const config = {
    runID: uuidv4(),
    k6Version: exec.version || 'unknown',
    seed: usedSeed,
    rpcUrl: RPC_URL,
    useGzip: USE_GZIP,
    epochSource: epochSource,
    epochsRaw: EPOCHS_RAW || null,
    epochsUsed: EPOCHS_PARSED,
    blockRanges: blockRanges,
    staticRpcParams: staticRpcParams,
  };

  return config;
}

// --- Main Test Logic ---
export default function (data) {
  // The logic inside the default function remains the same.
  // It defines the actions of a single VU.
  const perIterationSeed =
    data.seed + exec.vu.idInTest * 1000000 + exec.vu.iterationInScenario;
  const prng = new PRNG(perIterationSeed);

  const selectedRange =
    data.blockRanges[Math.floor(prng.random() * data.blockRanges.length)];

  const randomBlock =
    Math.floor(prng.random() * (selectedRange.max - selectedRange.min + 1)) +
    selectedRange.min;

  const payload = JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'getBlock',
    params: [randomBlock, data.staticRpcParams],
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  if (data.useGzip) {
    params.compression = 'gzip';
  }

  const res = http.post(data.rpcUrl, payload, params);

  // Record metrics
  responseSize.add(res.body.length);
  vu_latency_trend.add(res.timings.duration);

  // Perform standard response checks
  const httpSuccess = check(res, {
    'HTTP status is 200': (r) => r.status === 200,
  });

  if (httpSuccess) {
    try {
      const body = res.json();
      const rpcSuccess = check(body, {
        'RPC: no error field OR is a known acceptable error': (b) => {
          if (!b || !b.error) return true;
          if (
            b.error.message &&
            b.error.message.includes(
              'was skipped, or missing in long-term storage',
            )
          )
            return true;
          return false;
        },
      });

      if (!rpcSuccess) {
        rpcErrors.add(1);
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
      rpcErrors.add(1);
      console.error(
        `Failed to parse JSON response for block ${randomBlock}. Error: ${e}. Body: ${res.body}`,
      );
    }
  } else {
    rpcErrors.add(1);
  }

  sleep(1);
}

// --- Summary Function ---
export function handleSummary(data) {
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

  function formatNumber(num) {
    if (num === null || num === undefined || !isFinite(num)) return 'N/A';
    return num.toFixed(2).toLocaleString();
  }

  const green = (text) => `\x1b[32m${text}\x1b[0m`;
  const red = (text) => `\x1b[31m${text}\x1b[0m`;

  let summary = ['\n\n█ THRESHOLDS\n'];

  if (data && data.metrics) {
    for (const metricName in data.metrics) {
      if (data.metrics[metricName].thresholds) {
        summary.push(`\n  ${metricName}`);
        for (const thresholdName in data.metrics[metricName].thresholds) {
          const threshold = data.metrics[metricName].thresholds[thresholdName];
          const pass = threshold.ok;
          const symbol = pass ? green('✓') : red('✗');
          let valueStr = '';
          const metric = data.metrics[metricName];
          if (metric.type === 'trend') {
            const pValue = thresholdName.match(/p\((\d+\.?\d*)\)/);
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
          summary.push(`    ${symbol} '${thresholdName}' ${valueStr}`);
        }
      }
    }
  }

  summary.push('\n\n█ TOTAL RESULTS\n');

  const checks = data.metrics.checks;
  if (checks && checks.values) {
    summary.push(
      `\n  checks.........................: ${(
        (checks.values.passes / (checks.values.passes + checks.values.fails)) *
        100
      ).toFixed(2)}%   ${green('✓ ' + checks.values.passes)}   ${red(
        '✗ ' + checks.values.fails,
      )}`,
    );
  }

  const sortedMetricNames = Object.keys(data.metrics).sort();

  for (const name of sortedMetricNames) {
    const metric = data.metrics[name];
    if (name === 'checks' || !metric.values) continue;
    let line = `\n  ${name}......................:`;
    const isDataMetric = metric.contains === 'data' || name === 'response_size';
    const isCounterMetric = metric.type === 'counter';
    const formatter = isDataMetric
      ? formatBytes
      : isCounterMetric
      ? formatNumber
      : formatDuration;

    if (metric.type === 'trend') {
      line += ` avg=${formatter(metric.values.avg)} min=${formatter(
        metric.values.min,
      )} med=${formatter(metric.values.med)} max=${formatter(
        metric.values.max,
      )} p(90)=${formatter(metric.values['p(90)'])} p(95)=${formatter(
        metric.values['p(95)'],
      )}`;
    } else if (metric.type === 'counter') {
      line += ` ${formatter(metric.values.count)}   ${formatter(
        metric.values.rate,
      )}/s`;
    } else if (metric.type === 'gauge') {
      if (metric.values.min === metric.values.max) {
        line += ` value=${metric.values.value}`;
      } else {
        line += ` value=${metric.values.value} min=${metric.values.min} max=${metric.values.max}`;
      }
    }
    summary.push(line);
  }

  const finalConfig = data.setup_data;
  delete data.setup_data;

  const fullSummary = {
    configuration: finalConfig,
    results: data,
  };

  const timestamp = new Date()
    .toISOString()
    .replace(/:/g, '-')
    .replace(/\..+/, '');
  const runID = finalConfig ? finalConfig.runID : 'unknown-run';
  const jsonFilename = `summary-${timestamp}-${runID}.json`;

  console.log(`\nSaving detailed summary to ${jsonFilename}...`);

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

// 3. To run with an external controller, you must first start k6 in a paused state:
//    k6 run --paused k6-getBlock.js
//
// 4. Then, in a separate terminal, run the controller script (see the controller.js example).
//    The controller will connect to the k6 API and manage the test.

// 5. To reproduce a specific run, use the SEED from the logs:
//    k6 run --paused -e SEED=123456789 k6-getBlock.js

// --- Correlation Analysis ---
// To correlate response size and latency, you must export the raw results to a file
// and analyze it with an external tool (e.g., Python with pandas/matplotlib, R, etc.).
//
// 6. Run the test and output to a JSON file; the default filename will be `summary-<timestamp>-<runID>.json`.
//    k6 run --out json=results.json k6-getBlock.js
//
// 7. You can then parse `results.json`. Each line is a JSON object. Look for objects
//    where `type` is "Point" and `metric` is `http_req_duration` or `response_size`.
//    You can then match these points by their timestamp (`data.time`) to correlate them.
//
// 8. Run the load test with a live dashboard:
//    K6_WEB_DASHBOARD=true k6 run k6-getBlock.js
//
//    Customize the refresh rate of the dashboard:
//    K6_WEB_DASHBOARD_PERIOD=1s K6_WEB_DASHBOARD=true k6 run k6-getBlock.js
//
// 9. Export dashboard html to a file: K6_WEB_DASHBOARD_EXPORT={filename}; e.g. the filename can contain the timestamp;
//    K6_WEB_DASHBOARD=true K6_WEB_DASHBOARD_EXPORT=dashboard-$(date +%Y-%m-%dT%H-%M%S).html k6 run -e USE_GZIP=false k6-getBlock.js

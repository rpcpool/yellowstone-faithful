// controller.js
//
// This Node.js script acts as an external controller for a k6 test.
// It uses the k6 REST API to:
// 1. Start a paused test.
// 2. Incrementally ramp up VUs in steps to find the maximum sustainable load.
// 3. If a load level fails, it reverts to the last stable level and continues searching with a smaller increment.
// 4. When the increment is 1 and a failure occurs, it concludes that the max VUs have been found.
// 5. Stops the test when finished.

// --- Configuration ---
const K6_API_URL = 'http://localhost:6565';
const METRIC_TO_WATCH = 'http_req_duration';
const LATENCY_THRESHOLD_MS = 500; // Must match the threshold in the k6 script

// Configuration for incremental ramp-up
const INITIAL_VU_INCREMENT = 10; // How many VUs to add in each step initially
const HOLD_PER_STEP_SECONDS = 20; // How long to hold at each new VU level before checking metrics
const MAX_VUS_TO_TEST = 1000; // The maximum number of VUs the controller will attempt to reach

const FINAL_STABILITY_HOLD_SECONDS = 20; // How long to hold at the final stable point

// Helper function for making API calls to k6
async function k6api(path, method = 'GET', body = null) {
  const url = `${K6_API_URL}${path}`;
  try {
    const options = {
      method,
      headers: { 'Content-Type': 'application/json' },
    };
    if (body) {
      options.body = JSON.stringify(body);
    }
    const response = await fetch(url, options);

    if (!response.ok) {
      const errorBody = await response.text();
      console.error(
        `k6 API error: ${response.status} ${response.statusText}`,
        `Response Body: ${errorBody}`,
      );
      throw new Error(`k6 API error: ${response.status}`);
    }

    try {
      return await response.json();
    } catch (e) {
      return null;
    }
  } catch (error) {
    console.error(`Error calling k6 API at ${url}:`, error.message);
    if (error.cause && error.cause.code === 'ECONNREFUSED') {
      console.error(
        'Connection refused. Is k6 running in another terminal with the --paused flag?',
      );
      process.exit(1);
    }
    return null;
  }
}

// Helper function to set the number of VUs
async function setVUs(count) {
  const roundedCount = Math.round(count);
  console.log(`Setting VUs to ${roundedCount}...`);
  await k6api('/v1/status', 'PATCH', {
    data: { attributes: { vus: roundedCount } },
  });
}

// Helper function to sleep for a given number of milliseconds
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// Helper function to get the current metrics from the k6 API
async function getMetrics() {
  const metricsResponse = await k6api('/v1/metrics');
  if (!metricsResponse || !Array.isArray(metricsResponse.data)) {
    return { p95: null, reqsRate: 0 };
  }
  const latencyMetric = metricsResponse.data.find(
    (m) => m.id === METRIC_TO_WATCH,
  );
  const reqsMetric = metricsResponse.data.find((m) => m.id === 'http_reqs');
  const p95 = latencyMetric?.attributes?.sample?.['p(95)'];
  const reqsRate = reqsMetric?.attributes?.sample?.rate || 0;
  return { p95, reqsRate };
}

// Function to check latency against the threshold
async function checkLatencyThreshold() {
  console.log(`Checking latency metrics...`);
  const { p95, reqsRate } = await getMetrics();

  if (p95 === null) {
    console.warn(
      `Metric '${METRIC_TO_WATCH}' or its p(95) value is not yet available. Skipping check.`,
    );
    return true; // Return true (pass) if the specific metric isn't ready
  }

  console.log(
    `Current p(95) for ${METRIC_TO_WATCH} is ${p95.toFixed(
      2,
    )}ms | req/s: ${reqsRate.toFixed(2)}`,
  );

  if (p95 > LATENCY_THRESHOLD_MS) {
    console.error(
      `!!! LATENCY THRESHOLD BREACHED (${p95.toFixed(
        2,
      )}ms > ${LATENCY_THRESHOLD_MS}ms) !!!`,
    );
    return false; // Return false (fail)
  }

  return true; // Return true (pass)
}

// Main controller logic
async function main() {
  console.log('--- k6 External Controller ---');

  // 1. Check if k6 is running and paused
  const initialStatus = await k6api('/v1/status');
  if (!initialStatus || !initialStatus.data.attributes.paused) {
    console.error(
      'Controller requires k6 to be running with the --paused flag.',
    );
    console.log(
      'Please run `k6 run --paused k6-getBlock.js` in another terminal first.',
    );
    return;
  }
  console.log('k6 is running and paused. Starting test...');

  // 2. Resume the test
  await k6api('/v1/status', 'PATCH', {
    data: { attributes: { paused: false } },
  });

  // 3. Execute incremental ramp-up
  let currentVUs = 0;
  let lastKnownGoodVUs = 0;
  let vuIncrement = INITIAL_VU_INCREMENT;

  while (currentVUs < MAX_VUS_TO_TEST) {
    currentVUs += vuIncrement;
    console.log(
      `\n--- Ramping up to ${currentVUs} VUs (increment: ${vuIncrement}) ---`,
    );
    await setVUs(currentVUs);

    console.log(
      `Holding at ${currentVUs} VUs for ${HOLD_PER_STEP_SECONDS} seconds...`,
    );
    await sleep(HOLD_PER_STEP_SECONDS * 1000);

    const passed = await checkLatencyThreshold();

    if (passed) {
      console.log(`Step passed. ${currentVUs} VUs is a stable level.`);
      lastKnownGoodVUs = currentVUs;
    } else {
      // If a step fails, revert to the last good VU count,
      // reduce the increment size, and let the loop continue from there.
      // This creates a "binary search" style approach to find the tipping point.

      // If the increment is already 1 and we fail, we've found the tipping point.
      if (vuIncrement <= 1) {
        console.log(
          `Threshold failed at increment 1. Maximum stable load is ${lastKnownGoodVUs} VUs.`,
        );
        await setVUs(lastKnownGoodVUs); // Revert to the last good state
        break; // Exit the main loop
      }

      console.error(`Threshold failed at ${currentVUs} VUs.`);

      // Halve the increment for the next ramp-up attempt
      vuIncrement = Math.ceil(vuIncrement / 2);
      console.log(
        `\n---> Next ramp-up increment will be ${vuIncrement} VUs. Reverting to last known good level. <---`,
      );

      // Reset currentVUs to the last stable point to continue ramping up from there
      await setVUs(lastKnownGoodVUs);
      currentVUs = lastKnownGoodVUs;
    }
  }

  // 4. Hold at the final stable point and then ramp down
  console.log(
    `\nTest sequence finished. Final stable load was ${lastKnownGoodVUs} VUs.`,
  );
  console.log(
    `Holding at final stable point for ${FINAL_STABILITY_HOLD_SECONDS} seconds...`,
  );
  await sleep(FINAL_STABILITY_HOLD_SECONDS * 1000);

  // Perform a final check at the stable point
  console.log('\n--- Final Performance Check at Stable Load ---');
  await checkLatencyThreshold();
  console.log('------------------------------------------');

  console.log('\nRamping down VUs...');
  await setVUs(0);
  console.log('Stopping test...');
  await k6api('/v1/status', 'PATCH', {
    data: { attributes: { stopped: true } },
  });
  console.log('--- Controller finished ---');
}

main().catch((err) => {
  console.error('Controller script failed:', err);
});

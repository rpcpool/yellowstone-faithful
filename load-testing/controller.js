// controller.js
//
// This Node.js script acts as an external controller for a k6 test.
// It uses the k6 REST API to:
// 1. Start a paused test.
// 2. Incrementally ramp up VUs in steps.
// 3. After each step, it holds and checks performance metrics.
// 4. If latency exceeds a threshold, it incrementally steps down the VU count to find a stable level.
// 5. Stops the test when finished.

// --- Configuration ---
const K6_API_URL = 'http://localhost:6565';
const METRIC_TO_WATCH = 'http_req_duration';
const LATENCY_THRESHOLD_MS = 2000; // Must match the threshold in the k6 script

// Configuration for incremental ramp-up
const VU_INCREMENT = 10; // How many VUs to add in each step
const HOLD_PER_STEP_SECONDS = 15; // How long to hold at each new VU level before checking metrics
const MAX_VUS_TO_TEST = 1000; // The maximum number of VUs the controller will attempt to reach

// Configuration for downward search when a threshold fails
const VU_DECREMENT = 5; // How many VUs to remove when searching down
// **MODIFIED**: Increased hold time to allow p(95) metric to stabilize after a load change.
const HOLD_PER_DECREMENT_SECONDS = 15; // How long to hold at each lower VU level
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

// Function to check latency against the threshold
async function checkLatencyThreshold() {
  console.log(`Checking latency metrics...`);
  const metricsResponse = await k6api('/v1/metrics');

  if (!metricsResponse || !Array.isArray(metricsResponse.data)) {
    console.warn(
      'Metrics array not available in API response. Skipping check.',
    );
    return true; // Return true (pass) if we can't get metrics
  }

  const latencyMetric = metricsResponse.data.find(
    (m) => m.id === METRIC_TO_WATCH,
  );

  if (
    !latencyMetric ||
    !latencyMetric.attributes.sample ||
    latencyMetric.attributes.sample['p(95)'] === undefined
  ) {
    console.warn(
      `Metric '${METRIC_TO_WATCH}' or its p(95) value is not yet available. Skipping check.`,
    );
    return true; // Return true (pass) if the specific metric isn't ready
  }

  const p95 = latencyMetric.attributes.sample['p(95)'];
  console.log(`Current p(95) for ${METRIC_TO_WATCH} is ${p95.toFixed(2)}ms`);

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

  while (currentVUs < MAX_VUS_TO_TEST) {
    currentVUs += VU_INCREMENT;
    console.log(`\n--- Ramping up to ${currentVUs} VUs ---`);
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
      // Threshold breached, start searching downwards to find a stable point.
      console.error(
        `Threshold failed at ${currentVUs} VUs. Searching for a stable level by decrementing...`,
      );
      let stablePointFound = false;
      let searchVUs = currentVUs;

      while (!stablePointFound && searchVUs > 0) {
        searchVUs -= VU_DECREMENT;
        if (searchVUs <= 0) {
          searchVUs = 0;
          break; // Stop if we reach zero
        }

        console.log(`\n--- Stepping down to ${searchVUs} VUs ---`);
        await setVUs(searchVUs);

        console.log(
          `Holding at ${searchVUs} VUs for ${HOLD_PER_DECREMENT_SECONDS} seconds...`,
        );
        await sleep(HOLD_PER_DECREMENT_SECONDS * 1000);

        const innerPassed = await checkLatencyThreshold();
        if (innerPassed) {
          console.log(`Stable point found at ${searchVUs} VUs.`);
          lastKnownGoodVUs = searchVUs;
          stablePointFound = true;

          console.log(
            `Holding at stable point of ${lastKnownGoodVUs} VUs for ${FINAL_STABILITY_HOLD_SECONDS} seconds...`,
          );
          await sleep(FINAL_STABILITY_HOLD_SECONDS * 1000);
        }
      }

      if (!stablePointFound) {
        console.error('Could not find a stable VU level. Ramping down to 0.');
        lastKnownGoodVUs = 0;
      }

      break; // Exit the main ramp-up loop because the test objective is complete.
    }
  }

  // 4. Ramp down and stop the test
  console.log('\nTest sequence finished. Ramping down VUs.');
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

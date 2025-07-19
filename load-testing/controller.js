// controller.js
//
// This Node.js script acts as an external controller for a k6 test.
// It uses the k6 REST API to:
// 1. Start a paused test.
// 2. Dynamically adjust the number of VUs based on real-time latency metrics.
// 3. When latency is low, it ramps up faster. As latency approaches the threshold, it ramps up slower.
// 4. If latency exceeds the threshold, it ramps down proportionally to the overshoot.
// 5. The test concludes when this feedback loop finds the maximum sustainable VU level.

// --- Configuration ---
const K6_API_URL = 'http://localhost:6565';
const METRIC_TO_WATCH = 'http_req_duration';
const LATENCY_THRESHOLD_MS = 500; // The desired p(95) latency threshold.

// Configuration for dynamic VU adjustment
const MAX_VU_INCREMENT = 20; // The largest number of VUs to add in a single step (when latency is very low).
const VU_DECREMENT_BASE = 15; // A base multiplier to determine how much to decrement when latency is high.
const HOLD_PER_STEP_SECONDS = 20; // How long to hold at each VU level to measure performance.
const MAX_VUS_TO_TEST = 1000; // A safety limit for the maximum VUs to test.
const INITIAL_VUS = 10; // The number of VUs to start the test with.
const FINAL_STABILITY_HOLD_SECONDS = 20; // How long to hold at the final stable point.

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

// Main controller logic
async function main() {
  console.log('--- k6 External Controller ---');
  console.log(`Desired latency threshold (p95): ${LATENCY_THRESHOLD_MS}ms`);

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

  // 3. Execute dynamic VU adjustment loop
  let currentVUs = INITIAL_VUS;
  let lastKnownGoodVUs = 0;

  while (currentVUs > 0 && currentVUs < MAX_VUS_TO_TEST) {
    console.log(`\n--- Testing ${currentVUs} VUs ---`);
    await setVUs(currentVUs);

    console.log(`Holding for ${HOLD_PER_STEP_SECONDS} seconds...`);
    await sleep(HOLD_PER_STEP_SECONDS * 1000);

    const { p95, reqsRate } = await getMetrics();

    if (p95 === null) {
      console.warn(`Metric '${METRIC_TO_WATCH}' not available. Holding...`);
      continue;
    }

    console.log(
      `Current p(95) for ${METRIC_TO_WATCH} is ${p95.toFixed(
        2,
      )}ms | req/s: ${reqsRate.toFixed(2)}`,
    );

    if (p95 < LATENCY_THRESHOLD_MS) {
      // System is healthy, ramp up
      lastKnownGoodVUs = currentVUs;
      const headroom = (LATENCY_THRESHOLD_MS - p95) / LATENCY_THRESHOLD_MS;
      const increment = Math.max(1, Math.ceil(headroom * MAX_VU_INCREMENT));
      console.log(
        `Step passed. Latency headroom is ${(headroom * 100).toFixed(
          0,
        )}%. Ramping up by ${increment} VUs.`,
      );
      currentVUs += increment;
    } else {
      // System is overloaded, ramp down
      const overshoot = p95 - LATENCY_THRESHOLD_MS;
      const overshootPercent = (overshoot / LATENCY_THRESHOLD_MS) * 100;
      const overshootRatio = overshoot / LATENCY_THRESHOLD_MS;
      const decrement = Math.max(
        1,
        Math.ceil(overshootRatio * VU_DECREMENT_BASE),
      );

      console.error(
        `!!! LATENCY THRESHOLD BREACHED (overshoot by ${overshoot.toFixed(
          2,
        )}ms / ${overshootPercent.toFixed(2)}%) !!!`,
      );
      console.log(`Ramping down by ${decrement} VUs.`);
      currentVUs -= decrement;

      // If we decrement to or below our last stable point, we've found the peak.
      if (currentVUs <= lastKnownGoodVUs) {
        console.log(
          `Search has dropped to or below the last stable point. Concluding that ${lastKnownGoodVUs} is the maximum stable load.`,
        );
        break; // Exit the loop
      }
    }
  }

  // 4. Hold at the final stable point and then ramp down
  console.log(
    `\nTest sequence finished. Final stable load was ${lastKnownGoodVUs} VUs.`,
  );
  await setVUs(lastKnownGoodVUs);
  console.log(
    `Holding at final stable point for ${FINAL_STABILITY_HOLD_SECONDS} seconds...`,
  );
  await sleep(FINAL_STABILITY_HOLD_SECONDS * 1000);

  // Perform a final check at the stable point
  console.log('\n--- Final Performance Check at Stable Load ---');
  const { p95, reqsRate } = await getMetrics();
  if (p95 !== null) {
    console.log(
      `Final p(95) for ${METRIC_TO_WATCH} is ${p95.toFixed(
        2,
      )}ms | req/s: ${reqsRate.toFixed(2)}`,
    );
  }
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

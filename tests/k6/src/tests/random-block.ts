import { check, sleep } from 'k6';
import http from 'k6/http';
import { getBlockPayload } from '../payloads.ts';
import { GetSlotRPCResponse } from '../types.ts';
import { getRandomSlot, parseResponseBody } from '../utils.ts';

export const options = {
  iterations: 10,
  thresholds: {
    // Add thresholds for our checks
    checks: ['rate>=0.99'], // 99% of checks must pass
  },
};

const TEST_RPC_ENDPOINT = __ENV.TEST_RPC_ENDPOINT

if (!TEST_RPC_ENDPOINT) {
  throw new Error('TEST_RPC_ENDPOINT environment variable must be set');
}

export default function () {
  const headers = {
    'Content-Type': 'application/json',
  };

  const slot = getRandomSlot(0, 209520022);
  const payload = getBlockPayload(
    slot,
    'finalized',
    0
  );

  const response = http.post(TEST_RPC_ENDPOINT, JSON.stringify(payload), { headers });

  const checkResult = check(response, {
    'is status 200': (r) => r.status === 200,
  });

  if (!checkResult) {
    console.error(`HTTP request failed with status ${response.status}`);
    return;
  }

  try {
    if (response.body) {
      const bodyStr = parseResponseBody(response.body);
      const data = JSON.parse(bodyStr) as GetSlotRPCResponse;
      
      check(response, {
        'has valid JSON response': () => true,
        'no RPC error': () => !data.error,
      });

      if (data.error) {
        console.error(`RPC Error for slot ${slot}:`, JSON.stringify(data.error, null, 2));
      }
    } else {
      console.error('Response body is empty');
      check(response, {
        'has valid JSON response': () => false,
      });
    }
  } catch (error) {
    console.error('Failed to parse JSON response:', error);
    check(response, {
      'has valid JSON response': () => false,
    });
  }

  sleep(1);
}
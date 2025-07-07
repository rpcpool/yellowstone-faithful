import { check, sleep } from 'k6';
import http from 'k6/http';
import 'process';
import { getBlockPayload } from '../payloads.ts';
import { GetBlockRPCResponse } from '../types.ts';
import { getRandomSlot, parseResponseBody } from '../utils.ts';

export const options = {
  iterations: 10,
  thresholds: {
    checks: ['rate>=0.99'], // 99% of checks must pass
  },
};

const TEST_RPC_ENDPOINT = __ENV.TEST_RPC_ENDPOINT;
const TRUTH_RPC_ENDPOINT = __ENV.TRUTH_RPC_ENDPOINT;

if (!TEST_RPC_ENDPOINT || !TRUTH_RPC_ENDPOINT) {
  throw new Error('Both TEST_RPC_ENDPOINT and TRUTH_RPC_ENDPOINT environment variables must be set');
}

function deepEqual(obj1: any, obj2: any, path: string = ''): { equal: boolean; differences: string[] } {
  const differences: string[] = [];
  
  if (obj1 === obj2) {
    return { equal: true, differences };
  }
  
  if (obj1 === null || obj2 === null || obj1 === undefined || obj2 === undefined) {
    if (obj1 !== obj2) {
      differences.push(`${path}: ${obj1} !== ${obj2}`);
    }
    return { equal: differences.length === 0, differences };
  }
  
  if (typeof obj1 !== typeof obj2) {
    differences.push(`${path}: different types - ${typeof obj1} vs ${typeof obj2}`);
    return { equal: false, differences };
  }
  
  if (typeof obj1 !== 'object') {
    if (obj1 !== obj2) {
      differences.push(`${path}: ${obj1} !== ${obj2}`);
    }
    return { equal: differences.length === 0, differences };
  }
  
  if (Array.isArray(obj1) !== Array.isArray(obj2)) {
    differences.push(`${path}: one is array, other is not`);
    return { equal: false, differences };
  }
  
  if (Array.isArray(obj1)) {
    if (obj1.length !== obj2.length) {
      differences.push(`${path}: array lengths differ - ${obj1.length} vs ${obj2.length}`);
      return { equal: false, differences };
    }
    
    for (let i = 0; i < obj1.length; i++) {
      const result = deepEqual(obj1[i], obj2[i], `${path}[${i}]`);
      differences.push(...result.differences);
    }
  } else {
    const keys1 = Object.keys(obj1);
    const keys2 = Object.keys(obj2);
    const allKeys = new Set([...keys1, ...keys2]);
    
    for (const key of allKeys) {
      if (!(key in obj1)) {
        differences.push(`${path}.${key}: missing in test response`);
        continue;
      }
      if (!(key in obj2)) {
        differences.push(`${path}.${key}: missing in truth response`);
        continue;
      }
      
      const result = deepEqual(obj1[key], obj2[key], `${path}.${key}`);
      differences.push(...result.differences);
    }
  }
  
  return { equal: differences.length === 0, differences };
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

  // Make requests to both endpoints
  const testResponse = http.post(TEST_RPC_ENDPOINT, JSON.stringify(payload), { headers });
  const truthResponse = http.post(TRUTH_RPC_ENDPOINT, JSON.stringify(payload), { headers });

  // Check basic response status
  const testStatusOk = check(testResponse, {
    'test endpoint status 200': (r) => r.status === 200,
  });

  const truthStatusOk = check(truthResponse, {
    'truth endpoint status 200': (r) => r.status === 200,
  });

  if (!testStatusOk || !truthStatusOk) {
    console.error(`HTTP request failed - Test: ${testResponse.status}, Truth: ${truthResponse.status}`);
    return;
  }

  try {
    if (!testResponse.body || !truthResponse.body) {
      console.error('Empty response body received');
      check(false, {
        'valid response bodies': () => false,
      });
      return;
    }

    const testBodyStr = parseResponseBody(testResponse.body);
    const truthBodyStr = parseResponseBody(truthResponse.body);
    
    const testData = JSON.parse(testBodyStr) as GetBlockRPCResponse;
    const truthData = JSON.parse(truthBodyStr) as GetBlockRPCResponse;
    
    // Check for RPC errors
    check(testData, {
      'test endpoint no RPC error': () => !testData.error,
    });
    
    check(truthData, {
      'truth endpoint no RPC error': () => !truthData.error,
    });

    if (testData.error && truthData.error) {
      // Both have errors - check if they're the same
      const errorComparison = deepEqual(testData.error, truthData.error, 'error');
      check(errorComparison, {
        'same error response': () => errorComparison.equal,
      });
      
      if (!errorComparison.equal) {
        console.error(`Different errors for slot ${slot}:`);
        console.error('Test error:', JSON.stringify(testData.error, null, 2));
        console.error('Truth error:', JSON.stringify(truthData.error, null, 2));
        console.error('Differences:', errorComparison.differences);
      }
    } else if (testData.error || truthData.error) {
      // Only one has an error
      check(false, {
        'error consistency': () => false,
      });
      console.error(`Error mismatch for slot ${slot}:`);
      console.error('Test error:', testData.error ? JSON.stringify(testData.error, null, 2) : 'none');
      console.error('Truth error:', truthData.error ? JSON.stringify(truthData.error, null, 2) : 'none');
    } else {
      // Compare the actual block data
      const comparison = deepEqual(testData.result, truthData.result, 'result');
      check(comparison, {
        'identical block data': () => comparison.equal,
      });
      
      if (!comparison.equal) {
        console.error(`Block data mismatch for slot ${slot}:`);
        console.error(`Found ${comparison.differences.length} differences:`);
        comparison.differences.slice(0, 10).forEach(diff => console.error(`  - ${diff}`));
        if (comparison.differences.length > 10) {
          console.error(`  ... and ${comparison.differences.length - 10} more differences`);
        }
      }
    }
  } catch (error) {
    console.error('Failed to parse JSON responses:', error);
    check(false, {
      'valid JSON responses': () => false,
    });
  }

  sleep(1);
}
import { check, sleep, randomSeed } from 'k6';
import http from 'k6/http';
import { getBlockPayload } from '../payloads.ts';
import { GetBlockRPCResponse } from '../types.ts';
import { getRandomSlot, parseResponseBody } from '../utils.ts';
// @ts-ignore
import _ from 'https://cdn.skypack.dev/lodash@4';

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

function findDifferences(obj1: any, obj2: any, path: string = ''): string[] {
  const differences: string[] = [];
  
  if (_.isEqual(obj1, obj2)) {
    return differences;
  }
  
  if (obj1 === null || obj2 === null || obj1 === undefined || obj2 === undefined) {
    differences.push(`${path}: ${obj1} !== ${obj2}`);
    return differences;
  }
  
  if (typeof obj1 !== typeof obj2) {
    differences.push(`${path}: different types - ${typeof obj1} vs ${typeof obj2}`);
    return differences;
  }
  
  if (!_.isObject(obj1)) {
    differences.push(`${path}: ${obj1} !== ${obj2}`);
    return differences;
  }
  
  if (_.isArray(obj1) && _.isArray(obj2)) {
    if (obj1.length !== obj2.length) {
      differences.push(`${path}: array lengths differ - ${obj1.length} vs ${obj2.length}`);
      return differences;
    }
    
    _.forEach(obj1, (value, index) => {
      differences.push(...findDifferences(value, obj2[index], `${path}[${index}]`));
    });
  } else if (_.isPlainObject(obj1) && _.isPlainObject(obj2)) {
    const allKeys = _.union(_.keys(obj1), _.keys(obj2));
    
    _.forEach(allKeys, (key) => {
      if (!_.has(obj1, key)) {
        differences.push(`${path}.${key}: missing in test response`);
      } else if (!_.has(obj2, key)) {
        differences.push(`${path}.${key}: missing in truth response`);
      } else {
        differences.push(...findDifferences(obj1[key], obj2[key], `${path}.${key}`));
      }
    });
  } else if (_.isArray(obj1) !== _.isArray(obj2)) {
    differences.push(`${path}: one is array, other is not`);
  }
  
  return differences;
}

export default function () {
  // Set random seed if provided via environment variable
  const baseSeed = __ENV.K6_RANDOM_SEED;
  if (baseSeed) {
    // Combine base seed with iteration number for unique but reproducible randomness
    const iterationSeed = parseInt(baseSeed) + __ITER;
    randomSeed(iterationSeed);
  }

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
      const errorsEqual = _.isEqual(testData.error, truthData.error);
      check(errorsEqual, {
        'same error response': () => errorsEqual,
      });
      
      if (!errorsEqual) {
        console.error(`Different errors for slot ${slot}:`);
        console.error('Test error:', JSON.stringify(testData.error, null, 2));
        console.error('Truth error:', JSON.stringify(truthData.error, null, 2));
        const differences = findDifferences(testData.error, truthData.error, 'error');
        console.error('Differences:', differences);
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
      const blocksEqual = _.isEqual(testData.result, truthData.result);
      check(blocksEqual, {
        'identical block data': () => blocksEqual,
      });
      
      if (!blocksEqual) {
        console.error(`Block data mismatch for slot ${slot}:`);
        const differences = findDifferences(testData.result, truthData.result, 'result');
        console.error(`Found ${differences.length} differences:`);
        differences.slice(0, 10).forEach((diff: string) => console.error(`  - ${diff}`));
        if (differences.length > 10) {
          console.error(`  ... and ${differences.length - 10} more differences`);
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
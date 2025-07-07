import { check, sleep } from 'k6';
import http from 'k6/http';
import { Counter, Rate, Trend } from 'k6/metrics';
import { getSignaturesForAddressPayload } from '../payloads.ts';
import { GetSignaturesForAddressResponse } from '../types.ts';
import { getRandomSolanaAddress, parseResponseBody } from '../utils.ts';

// Custom metrics
const errorCounter = new Counter('rpc_errors');
const errorRate = new Rate('error_rate');
const requestDuration = new Trend('request_duration');
const successfulAddresses = new Counter('successful_addresses');
const emptyAddresses = new Counter('empty_addresses');
const signatureOrderingErrors = new Counter('signature_ordering_errors');

export const options = {
  iterations: 10,
  thresholds: {
    checks: ['rate>=0.8'],
    error_rate: ['rate<0.3'],
    request_duration: ['p(95)<5000'],
    'successful_addresses': ['count>0'],
    'signature_ordering_errors': ['count==0'],
    'empty_addresses': ['count<10'],
  },
  rps: 1,
};

const RPC_ENDPOINT = 'http://faithful5.rpcpool.wg:8897';
const MAX_PAGES = 3;
const SIGNATURES_PER_PAGE = 25;
const REQUEST_DELAY = 3;

const getBodyPreview = (body: string | ArrayBuffer | null): string => {
  if (!body) return 'empty';
  const str = parseResponseBody(body);
  return str.slice(0, 200);
};

export default function () {
  const headers = {
    'Content-Type': 'application/json',
    'Accept': 'application/json',
  };

  const address = getRandomSolanaAddress();
  let beforeSignature: string | undefined = undefined;
  let untilSignature: string | undefined = undefined; // Optional: for range testing
  let pageCount = 0;
  let totalSignatures = 0;
  let previousSignatures: string[] = [];

  console.log(`Starting test for address: ${address}`);

  while (pageCount < MAX_PAGES) {
    const payload = getSignaturesForAddressPayload(
      address,
      beforeSignature,
      untilSignature,
      SIGNATURES_PER_PAGE,
      'finalized'
    );

    const payloadStr = JSON.stringify(payload);
      
    console.log(`Request ${pageCount + 1}:`, {
      endpoint: RPC_ENDPOINT,
      method: payload.method,
      address: payload.params[0],
      before: beforeSignature,
      limit: SIGNATURES_PER_PAGE
    });

    const startTime = new Date();
    const response = http.post(RPC_ENDPOINT, payloadStr, {
      headers: {
        ...headers,
        'Content-Length': payloadStr.length.toString()
      },
      timeout: '15s'
    });
    const duration = new Date().getTime() - startTime.getTime();
    requestDuration.add(duration);

    console.log(`Response ${pageCount + 1} status: ${response.status}, time: ${duration}ms`);

    const checkResult = check(response, {
      'is status 200': (r) => r.status === 200,
    });

    if (!checkResult) {
      console.error(`HTTP request failed:`, {
        status: response.status,
        headers: response.headers,
        body: getBodyPreview(response.body),
        error: response.error
      });
      errorCounter.add(1);
      errorRate.add(true);
      break;
    }

    try {
      if (!response.body) {
        throw new Error('Empty response body');
      }

      const bodyStr = parseResponseBody(response.body);
      const data = JSON.parse(bodyStr) as GetSignaturesForAddressResponse;
        
      check(response, {
        'has valid JSON response': () => true,
        'no RPC error': () => data.error === null || data.error === undefined,
        'has signatures array': () => Array.isArray(data.result),
      });

      if (data.error !== null && data.error !== undefined) {
        console.error(`RPC Error for address ${address}:`, {
          error: data.error,
          errorCode: data.error.code,
          errorMessage: data.error.message,
          requestPayload: payload,
          responseTime: duration
        });
        errorCounter.add(1);
        errorRate.add(true);
        
        // Handle specific RPC error codes
        if (data.error.code === -32602) {
          console.error('Invalid parameters provided');
        } else if (data.error.code === -32001) {
          console.error('Resource unavailable');
        }
        break;
      } 
        
      errorRate.add(false);
        
      if (data.result) {
        const signatures = data.result;
        totalSignatures += signatures.length;
          
        console.log(
          `Page ${pageCount + 1} success:`,
          `Address: ${address}`,
          `Found: ${signatures.length}`,
          `Total: ${totalSignatures}`,
          `Time: ${duration}ms`
        );

        // Validate signature ordering (newest to oldest)
        let orderingValid = true;
        for (let i = 1; i < signatures.length; i++) {
          if (signatures[i].slot > signatures[i - 1].slot) {
            orderingValid = false;
            console.error(`Signature ordering error at index ${i}:`, {
              previous: { signature: signatures[i - 1].signature, slot: signatures[i - 1].slot },
              current: { signature: signatures[i].signature, slot: signatures[i].slot }
            });
            signatureOrderingErrors.add(1);
            break;
          }
        }
        
        check(response, {
          'signatures ordered correctly': () => orderingValid,
        });

        // Check for duplicates across pages
        const currentSignatures = signatures.map(s => s.signature);
        const duplicates = currentSignatures.filter(sig => previousSignatures.includes(sig));
        if (duplicates.length > 0) {
          console.error(`Found ${duplicates.length} duplicate signatures across pages`);
        }
        previousSignatures = [...previousSignatures, ...currentSignatures];

        if (pageCount === 0 && signatures.length === 0) {
          emptyAddresses.add(1);
          console.log(`Address ${address} has no signatures`);
        } else if (pageCount === 0) {
          successfulAddresses.add(1);
        }

        if (signatures.length < SIGNATURES_PER_PAGE) {
          console.log('Reached end of signatures list');
          break;
        }

        beforeSignature = signatures[signatures.length - 1]?.signature;
        if (!beforeSignature) {
          console.log('No valid signature for pagination');
          break;
        }
      } else {
        console.log('No results in response');
        break;
      }
    } catch (error) {
      const err = error as Error;
      console.error('Response parsing error:', {
        error: err.message,
        bodyPreview: getBodyPreview(response.body)
      });
      errorCounter.add(1);
      errorRate.add(true);
      break;
    }

    pageCount++;
    console.log(`Sleeping ${REQUEST_DELAY}s before next request...`);
    sleep(REQUEST_DELAY);
  }

  console.log(`Test finished for ${address}:`, {
    pagesProcessed: pageCount,
    totalSignatures
  });
}
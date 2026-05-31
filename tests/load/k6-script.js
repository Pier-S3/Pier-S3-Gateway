import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('error_rate');
const authOverhead = new Trend('auth_overhead_ms');

export const options = {
  stages: [
    { duration: '30s', target: 100 },   // ramp up
    { duration: '5m', target: 500 },    // sustain 500 RPS
    { duration: '30s', target: 0 },     // ramp down
  ],
  thresholds: {
    'http_req_duration{scenario:default}': ['p(99)<5'],  // p99 < 5ms overhead
    'error_rate': ['rate<0.001'],                         // < 0.1% error rate
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';
const TOKEN = __ENV.AUTH_TOKEN || 'test-token';

const headers = {
  Authorization: `Bearer ${TOKEN}`,
  'Content-Type': 'application/json',
};

export default function () {
  const scenario = Math.random();

  if (scenario < 0.6) {
    // 60% GET requests (list objects)
    const res = http.get(`${BASE_URL}/api/v1/buckets/test-bucket/objects?prefix=&delimiter=/`, { headers });
    check(res, {
      'GET objects status 200': (r) => r.status === 200,
    });
    errorRate.add(res.status !== 200);
    authOverhead.add(res.timings.waiting);
  } else if (scenario < 0.8) {
    // 20% GET requests (list buckets)
    const res = http.get(`${BASE_URL}/api/v1/buckets`, { headers });
    check(res, {
      'GET buckets status 200': (r) => r.status === 200,
    });
    errorRate.add(res.status !== 200);
    authOverhead.add(res.timings.waiting);
  } else if (scenario < 0.95) {
    // 15% PUT requests (upload small object)
    const payload = 'x'.repeat(1024); // 1 KB
    const res = http.put(
      `${BASE_URL}/api/v1/buckets/test-bucket/objects/load-test/${Date.now()}.txt`,
      payload,
      { headers: { ...headers, 'Content-Type': 'application/octet-stream' } },
    );
    check(res, {
      'PUT object status 200': (r) => r.status === 200,
    });
    errorRate.add(res.status !== 200);
    authOverhead.add(res.timings.waiting);
  } else {
    // 5% HEAD requests (object metadata)
    const res = http.get(`${BASE_URL}/api/v1/buckets/test-bucket/objects/test-file.txt/meta`, { headers });
    check(res, {
      'HEAD meta status 200 or 404': (r) => r.status === 200 || r.status === 404,
    });
    errorRate.add(res.status >= 500);
    authOverhead.add(res.timings.waiting);
  }
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: '  ', enableColors: true }),
    'tests/load/results.json': JSON.stringify(data, null, 2),
  };
}

function textSummary(data, opts) {
  const metrics = data.metrics;
  let output = '\n=== S3 Proxy Gateway Load Test Results ===\n\n';
  output += `Total Requests: ${metrics.http_reqs?.values?.count || 0}\n`;
  output += `Error Rate: ${((metrics.error_rate?.values?.rate || 0) * 100).toFixed(3)}%\n`;
  output += `p99 Latency: ${metrics.http_req_duration?.values?.['p(99)']?.toFixed(2) || 'N/A'} ms\n`;
  output += `Auth Overhead p99: ${metrics.auth_overhead_ms?.values?.['p(99)']?.toFixed(2) || 'N/A'} ms\n`;
  return output;
}

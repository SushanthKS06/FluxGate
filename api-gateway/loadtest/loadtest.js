import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const requestDuration = new Trend('request_duration');

export const options = {
  stages: [
    { duration: '30s', target: 100 },
    { duration: '1m', target: 500 },
    { duration: '2m', target: 1000 },
    { duration: '30s', target: 1000 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.05'],
    errors: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';
const TOKEN = __ENV.AUTH_TOKEN || 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c';

export default function () {
  group('API Gateway Load Test', () => {
    const headers = {
      'Authorization': `Bearer ${TOKEN}`,
      'Content-Type': 'application/json',
    };

    const res = http.get(`${BASE_URL}/api/test`, { headers });
    
    requestDuration.add(res.timings.duration);
    
    const success = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 500ms': (r) => r.timings.duration < 500,
      'has request ID': (r) => r.headers['X-Request-ID'] !== undefined,
    });

    errorRate.add(!success);
  });

  sleep(0.1);
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'summary.json': JSON.stringify(data),
  };
}

function textSummary(data, options) {
  const indent = options.indent || '';
  const enableColors = options.enableColors || false;
  
  let output = '\n' + indent + '===========================================\n';
  output += indent + '         API GATEWAY LOAD TEST RESULTS\n';
  output += indent + '===========================================\n\n';
  
  output += indent + `Total Requests: ${data.metrics.http_reqs.values.count}\n`;
  output += indent + `Failed Requests: ${data.metrics.http_req_failed.values.passes}\n`;
  output += indent + `Request Rate: ${data.metrics.http_reqs.values.rate.toFixed(2)} req/s\n\n`;
  
  output += indent + 'Latency Percentiles:\n';
  output += indent + `  p50: ${(data.metrics.http_req_duration.values['p(50)'] || 0).toFixed(2)}ms\n`;
  output += indent + `  p95: ${(data.metrics.http_req_duration.values['p(95)'] || 0).toFixed(2)}ms\n`;
  output += indent + `  p99: ${(data.metrics.http_req_duration.values['p(99)'] || 0).toFixed(2)}ms\n\n`;
  
  output += indent + '===========================================\n';
  
  return output;
}

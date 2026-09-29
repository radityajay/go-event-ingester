import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics.
const errorRate = new Rate('error_rate');
const ingestDuration = new Trend('ingest_duration', true);

// Test scenarios — ramp up to 10K events/sec target.
export const options = {
  scenarios: {
    // Scenario 1: Single event ingestion — sustained load.
    single_events: {
      executor: 'ramping-arrival-rate',
      startRate: 100,
      timeUnit: '1s',
      preAllocatedVUs: 200,
      maxVUs: 500,
      stages: [
        { duration: '30s', target: 1000 },   // Warm up to 1K/s
        { duration: '1m',  target: 5000 },   // Ramp to 5K/s
        { duration: '2m',  target: 10000 },  // Sustain 10K/s
        { duration: '30s', target: 0 },      // Cool down
      ],
      exec: 'singleEvent',
    },

    // Scenario 2: Batch ingestion — high throughput.
    batch_events: {
      executor: 'ramping-arrival-rate',
      startRate: 10,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 200,
      stages: [
        { duration: '30s', target: 50 },    // 50 batches/s × 100 = 5K events/s
        { duration: '1m',  target: 100 },   // 100 batches/s × 100 = 10K events/s
        { duration: '2m',  target: 100 },   // Sustain
        { duration: '30s', target: 0 },     // Cool down
      ],
      exec: 'batchEvents',
      startTime: '5m', // Start after single scenario.
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],  // p95 < 500ms, p99 < 1s
    error_rate: ['rate<0.01'],                        // < 1% error rate
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

const EVENT_TYPES = ['page_view', 'click', 'scroll', 'purchase', 'signup', 'login', 'logout', 'search', 'add_to_cart', 'checkout'];
const DEVICES = ['mobile', 'desktop', 'tablet'];
const COUNTRIES = ['US', 'ID', 'GB', 'DE', 'JP', 'BR', 'IN', 'FR', 'CA', 'AU'];
const PAGES = ['/home', '/pricing', '/about', '/dashboard', '/settings', '/profile', '/products', '/checkout', '/search', '/blog'];

function randomElement(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

function generateEvent() {
  return {
    user_id: `usr_${Math.floor(Math.random() * 100000)}`,
    event_type: randomElement(EVENT_TYPES),
    page: randomElement(PAGES),
    device: randomElement(DEVICES),
    country: randomElement(COUNTRIES),
  };
}

// Single event ingestion.
export function singleEvent() {
  const payload = JSON.stringify(generateEvent());

  const res = http.post(`${BASE_URL}/api/v1/events`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  const success = check(res, {
    'status is 202': (r) => r.status === 202,
  });

  errorRate.add(!success);
  ingestDuration.add(res.timings.duration);
}

// Batch event ingestion (100 events per batch).
export function batchEvents() {
  const batch = [];
  for (let i = 0; i < 100; i++) {
    batch.push(generateEvent());
  }

  const payload = JSON.stringify(batch);

  const res = http.post(`${BASE_URL}/api/v1/events/batch`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  const success = check(res, {
    'batch status is 202': (r) => r.status === 202,
  });

  errorRate.add(!success);
  ingestDuration.add(res.timings.duration);
}

// Quick smoke test — run with: k6 run --iterations 10 loadtest/k6.js
export default function () {
  singleEvent();
  sleep(0.1);
}

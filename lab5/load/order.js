import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '1m',  target: 10 },   // ramp up
        { duration: '15m', target: 10 },   // steady: держим 10 VU 15 минут
        { duration: '1m',  target: 0 },    // ramp down
    ],
    thresholds: {
        http_req_failed:   ['rate<0.01'],   // <1% ошибок
        http_req_duration: ['p(95)<300'],   // SLA: p95 < 300 мс
    },
};

export default function () {
    const res = http.post('http://127.0.0.1:30080/order', null, {
        headers: { 'Content-Type': 'application/json' },
        tags: { endpoint: 'order' },
    });

    check(res, {
        'status is 200': (r) => r.status === 200,
        'has id':        (r) => r.json('id') !== undefined,
    });

    sleep(1);
}
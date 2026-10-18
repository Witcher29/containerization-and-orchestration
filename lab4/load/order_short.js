import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '30s', target: 20 },   // ramp up
        { duration: '2m',  target: 20 },   // steady: держим 20 VU 2 минуты
        { duration: '30s', target: 0 },    // ramp down
    ],
    thresholds: {
        http_req_failed:   ['rate<0.01'],   // <1% ошибок
        http_req_duration: ['p(95)<300'],   // SLA: p95 < 300 мс
    },
};

export default function () {
    const res = http.post('http://localhost:8080/order', null, {
        headers: { 'Content-Type': 'application/json' },
        tags: { endpoint: 'order' },
    });

    check(res, {
        'status is 200': (r) => r.status === 200,
        'has id':        (r) => r.json('id') !== undefined,
    });

    sleep(0.5); // Уменьшили паузу, чтобы создать больше нагрузки
}
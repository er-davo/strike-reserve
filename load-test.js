import http from 'k6/http';
import { check, sleep } from 'k6';
import { randomItem } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

export const options = {
    stages: [
        { duration: '30s', target: 40 }, 
        { duration: '2m', target: 100 },
        { duration: '30s', target: 0 },
    ],
    thresholds: {
        http_req_failed: ['rate<0.01'],
        http_req_duration: ['p(95)<300'],
    },
};

const BASE_URL = 'http://localhost:8080';

export function setup() {
    const loginParams = { headers: { 'Content-Type': 'application/json' } };
    let tokens = [];

    console.log("Starting setup: logging in users...");

    // Логиним 100 случайных пользователей из наших 500 созданных
    for (let i = 0; i < 100; i++) {
        let userId = Math.floor(Math.random() * 500);
        let payload = JSON.stringify({
            email: `user${userId}@test.com`,
            password: "password123" // тот самый пароль из сида
        });

        let res = http.post(`${BASE_URL}/login`, payload, loginParams);
        
        if (res.status === 200) {
            tokens.push(res.json().token);
        } else {
            console.warn(`Failed to login user${userId}: ${res.status} ${res.body}`);
        }
    }

    if (tokens.length === 0) {
        throw new Error("No tokens acquired! Check your DB and seed data.");
    }

    // Получаем список комнат один раз для всех VU
    const lanesRes = http.get(`${BASE_URL}/lanes/list`, {
        headers: { Authorization: `Bearer ${tokens[0]}` },
    });

    if (lanesRes.status !== 200) {
        throw new Error(`Failed to get lanes: ${lanesRes.status} ${lanesRes.body}`);
    }

    const laneIds = lanesRes.json().lanes.map(lane => lane.id);

    return {
        tokens: tokens,
        laneIds: laneIds,
    };
}

export default function (data) {
    const token = randomItem(data.tokens);
    const userHeaders = { 
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json'
    };
    const randomLaneId = randomItem(data.laneIds);

    // Сценарий 1: Проверка свободных слотов (основная нагрузка)
    const targetDate = new Date();
    targetDate.setDate(targetDate.getDate() + Math.floor(Math.random() * 45));
    const dateString = targetDate.toISOString().split('T')[0];

    const slotsRes = http.get(
        `${BASE_URL}/lanes/${randomLaneId}/slots/list?date=${dateString}`, 
        { headers: userHeaders }
    );

    check(slotsRes, {
        'status is 200': (r) => r.status === 200,
    });

    // Сценарий 2: Попытка забронировать слот (с вероятностью 15%)
    if (Math.random() < 0.15) {
        const slots = slotsRes.json().slots;
        if (slots && slots.length > 0) {
            const randomSlot = randomItem(slots);
            
            const bookingRes = http.post(
                `${BASE_URL}/bookings/create`,
                JSON.stringify({ slotId: randomSlot.id }),
                { headers: userHeaders }
            );

            // ДОБАВЬ ЭТО:
            if (bookingRes.status !== 201 && bookingRes.status !== 409) {
                console.log(`❌ Ошибка бронирования: Статус ${bookingRes.status}, Тело: ${bookingRes.body}, SlotID: ${randomSlot.id}`);
            }

            check(bookingRes, {
                'booking response ok': (r) => [201, 409].includes(r.status),
            });
        }
    }
}
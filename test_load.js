import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

// Метрики
let authResponseTime = new Trend('auth_response_time');
let buyItemResponseTime = new Trend('buy_item_response_time');
let sendCoinResponseTime = new Trend('send_coin_response_time');
let infoResponseTime = new Trend('info_response_time');

// Конфигурация теста
export let options = {
    stages: [
        { duration: '30s', target: 100 },  
        { duration: '1m', target: 100 },   
        { duration: '30s', target: 0 },    
    ],
    thresholds: {
        auth_response_time: ['p(95)<50'], 
        buy_item_response_time: ['p(95)<50'], 
        send_coin_response_time: ['p(95)<50'], 
        info_response_time: ['p(95)<50'], 
    },
};

const BASE_URL = 'http://localhost:8080';
const ITEMS = ['t-shirt', 'cup', 'book', 'pen', 'powerbank', 'hoody', 'umbrella', 'socks', 'wallet', 'pink-hoody'];
const RECEIVER_USERNAME = 'receiver';
const RECEIVER_PASSWORD = 'password';

function auth(username, password) {
    let url = `${BASE_URL}/api/auth`;
    let payload = JSON.stringify({
        username: username,
        password: password,
    });
    let params = {
        headers: {
            'Content-Type': 'application/json',
        },
    };
    let res = http.post(url, payload, params);
    authResponseTime.add(res.timings.duration);
    check(res, {
        'auth status is 200': (r) => r.status === 200,
    });
    if (res.status === 200) {
        return res.json('token');
    }
    return null;
}

function buyItem(token, item) {
    let url = `${BASE_URL}/api/buy/${item}`;
    let params = {
        headers: {
            'Authorization': token,
        },
    };
    let res = http.get(url, params);
    buyItemResponseTime.add(res.timings.duration);
    check(res, {
        'buy item status is 200': (r) => r.status === 200,
    });
}

function sendCoin(token, toUser, amount) {
    let url = `${BASE_URL}/api/sendCoin`;
    let payload = JSON.stringify({
        toUser: toUser,
        amount: amount,
    });
    let params = {
        headers: {
            'Authorization': token,
            'Content-Type': 'application/json',
        },
    };
    let res = http.post(url, payload, params);
    sendCoinResponseTime.add(res.timings.duration);
    check(res, {
        'send coin status is 200': (r) => r.status === 200,
    });
}

function getInfo(token) {
    let url = `${BASE_URL}/api/info`;
    let params = {
        headers: {
            'Authorization': token,
        },
    };
    let res = http.get(url, params);
    infoResponseTime.add(res.timings.duration);
    check(res, {
        'get info status is 200': (r) => r.status === 200,
    });
}


export default function () {
    // Уникальное имя пользователя для каждого виртуального пользователя
    let username = `user_${__VU}`;
    let password = 'password';
    let token = auth(username, password);

    if (token) {
        // Покупка случайного товара
        let item = ITEMS[Math.floor(Math.random() * ITEMS.length)];
        buyItem(token, item);

        // // Передача монет предварительно созданному пользователю
        sendCoin(token, RECEIVER_USERNAME, 0);

        // Получение информации о пользователе
        getInfo(token);
    }

    sleep(0.05); 
}
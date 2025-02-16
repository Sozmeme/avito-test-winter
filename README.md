# avito-test-winter

## Usage

### Запуск сервиса
```sh
docker-compose up --build
```

### Тестирование
Запуск тестов:
```sh
go test -v
```

## Вопросы и ответы

### Можно ли сгенерировать код из данного в задании API?
API дан и структурирован – `swagger.io` позволяет сгенерировать сервер на основе данной конфигурации, поэтому использовал его. Оставалось реализовать логику ручек, связь с БД, авторизацию.

### Основные сущности в БД, их отношения?
Выделены сущности:
- **Пользователь (`users`)**
  - `id SERIAL PRIMARY KEY` – уникальный идентификатор пользователя
  - `username VARCHAR(255) UNIQUE NOT NULL` – имя пользователя (уникальное)
  - `password VARCHAR(255) NOT NULL` – хешированный пароль
  - `coins INT DEFAULT 1000` – количество монет у пользователя
  
- **Покупки (`purchases`)**
  - `id SERIAL PRIMARY KEY` – уникальный идентификатор покупки
  - `user_id INT REFERENCES users(id)` – связь с пользователем
  - `item_name VARCHAR(255) NOT NULL` – название купленного предмета
  - `created_at TIMESTAMP DEFAULT NOW()` – дата и время покупки

- **Транзакции (`transactions`)**
  - `id SERIAL PRIMARY KEY` – уникальный идентификатор транзакции
  - `sender_id INT REFERENCES users(id)` – отправитель
  - `receiver_id INT REFERENCES users(id)` – получатель
  - `amount INT NOT NULL` – сумма перевода
  - `created_at TIMESTAMP DEFAULT NOW()` – дата и время транзакции

**Связи:**
- `purchases.user_id` → `users.id` (один пользователь может иметь много покупок)
- `transactions.sender_id` и `transactions.receiver_id` → `users.id` (пользователи могут отправлять друг другу монеты)

### Как хранить/передавать JWT?
JWT хранится на стороне клиента, передается через заголовок `Authorization`. К каждому запросу на покупку, перевод или отображение данных прикрепляется JWT, который сервер парсит и использует для выполнения запросов к БД от лица пользователя.

### Как правильно сделать нагрузочное тестирование? Как ускорить работу сервиса?
Ранее не сталкивался с нагрузычными. Разобрался, написал скрипт `k6`, проверяющий сценарий авторизации пользователя и выполнение трёх типов запросов. Чтобы не возникало ошибок, когда у пользователей не хватает денег на перевод или покупку, сделал цену товаров 0 и установил возможность перевода 0 монет для теста.
- При `RPS = 100` всё работало стабильно, разве что send coin работал медленно. 
  ![image](https://github.com/user-attachments/assets/6736584c-65ae-43e4-a0fc-4529192c52a4)

- При `RPS = 1000` большинство запросов не выполнялось, поэтому добавил:
  - Кэширование токенов
  - Индексирование в БД
  - Использование горутин для обработки запросов к БД

После оптимизации стало лучше, но при высокой нагрузке остаются ошибки. Данный момент требует дополнительной проработки. Возможно проблема в скрипте, подходе тестирования.
![image](https://github.com/user-attachments/assets/368f0037-a59a-412e-bb7a-d1910c9b211d)


### Рефакторинг и тестирование - недочеты
Фокусировался на производительности для нагрузочных тестах, поэтому не хватило времени на полноценный рефакторинг кода. По неопытности допустил ошибку в unit-тестах и понял это под конец дедлайна – unit-тесты фактически взаимодействуют с реальной бд в Docker, что не соответствует принципу изоляции юнит-тестов. В идеале следовало использовать мок-базу данных (`sqlmock` или `testcontainers`). 

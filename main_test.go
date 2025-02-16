package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Функция для настройки переменных окружения для тестов
func setupTestEnvironment(t *testing.T) {
	os.Setenv("DB_HOST", "host.docker.internal")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "avito")
	os.Setenv("DB_PASSWORD", "secret")
	os.Setenv("DB_NAME", "avito_shop")

	var err error
	db, err = initdb()
	if err != nil {
		t.Fatalf("Не удалось инициализировать базу данных: %v", err)
	}
}

func cleanupTestEnvironment(t *testing.T) {
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_NAME")

	_, err := db.Exec("DELETE FROM transactions")
	if err != nil {
		t.Fatalf("Ошибка при очистке данных из таблицы transactions: %v", err)
	}

	_, err = db.Exec("DELETE FROM purchases")
	if err != nil {
		t.Fatalf("Ошибка при очистке данных из таблицы purchases: %v", err)
	}

	_, err = db.Exec("DELETE FROM users")
	if err != nil {
		t.Fatalf("Ошибка при очистке данных из таблицы users: %v", err)
	}
}

func performRequest(r http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf *bytes.Buffer
	if body != nil {
		buf = new(bytes.Buffer)
		json.NewEncoder(buf).Encode(body)
	}

	req, _ := http.NewRequest(method, path, buf)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)
	return recorder
}

// Тест аутентификации пользователя (POST /api/auth)
func TestApiAuthPost(t *testing.T) {
	// Настройка переменных окружения для тестов
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	router := NewRouter()

	// Создаем нового пользователя
	authRequest := AuthRequest{
		Username: "testuser",
		Password: "testpassword",
	}

	// Выполняем POST-запрос на /api/auth
	recorder := performRequest(router, "POST", "/api/auth", authRequest)

	// Проверяем статус код
	assert.Equal(t, http.StatusOK, recorder.Code, "Ожидался код 200")

	// Проверяем наличие токена в ответе
	var authResponse AuthResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &authResponse)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}
	assert.NotEmpty(t, authResponse.Token, "Токен должен быть не пустым")
}

// Тест покупки товара (GET /api/buy/{item})
func TestApiBuyItemGet(t *testing.T) {
	// Настройка переменных окружения для тестов
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	router := NewRouter()

	// Аутентифицируем пользователя для получения токена
	authRequest := AuthRequest{
		Username: "testuser",
		Password: "testpassword",
	}

	authRecorder := performRequest(router, "POST", "/api/auth", authRequest)
	var authResponse AuthResponse
	err := json.Unmarshal(authRecorder.Body.Bytes(), &authResponse)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}

	// Покупаем товар
	token := authResponse.Token
	itemName := "t-shirt" // Предмет из списка itemPrices

	req, _ := http.NewRequest("GET", "/api/buy/"+itemName, nil)
	req.Header.Set("Authorization", token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// Проверяем статус код
	assert.Equal(t, http.StatusOK, recorder.Code, "Ожидался код 200")

	// Проверяем сообщение об успешной покупке
	var responseMap map[string]string
	err = json.Unmarshal(recorder.Body.Bytes(), &responseMap)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}
	expectedMessage := "You bought a t-shirt!"
	assert.Equal(t, expectedMessage, responseMap["message"], "Неверное сообщение об успехе")
}

// Тест получения информации о пользователе (GET /api/info)
func TestApiInfoGet(t *testing.T) {
	// Настройка переменных окружения для тестов
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	router := NewRouter()

	// Аутентифицируем пользователя для получения токена
	authRequest := AuthRequest{
		Username: "testuser",
		Password: "testpassword",
	}

	authRecorder := performRequest(router, "POST", "/api/auth", authRequest)
	var authResponse AuthResponse
	err := json.Unmarshal(authRecorder.Body.Bytes(), &authResponse)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}

	// Получаем информацию о пользователе
	token := authResponse.Token
	req, _ := http.NewRequest("GET", "/api/info", nil)
	req.Header.Set("Authorization", token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// Проверяем статус код
	assert.Equal(t, http.StatusOK, recorder.Code, "Ожидался код 200")

	// Проверяем содержимое ответа
	var infoResponse InfoResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &infoResponse)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}
	assert.GreaterOrEqual(t, infoResponse.Coins, int32(0), "Количество монет должно быть неотрицательным")
}

// Тест отправки монет другому пользователю (POST /api/sendCoin)
func TestApiSendCoinPost(t *testing.T) {
	// Настройка переменных окружения для тестов
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)
	router := NewRouter()

	// Аутентифицируем первого пользователя
	authRequest1 := AuthRequest{
		Username: "testuser1",
		Password: "testpassword1",
	}
	authRecorder1 := performRequest(router, "POST", "/api/auth", authRequest1)
	var authResponse1 AuthResponse
	err := json.Unmarshal(authRecorder1.Body.Bytes(), &authResponse1)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}

	// Аутентифицируем второго пользователя
	authRequest2 := AuthRequest{
		Username: "testuser2",
		Password: "testpassword2",
	}
	authRecorder2 := performRequest(router, "POST", "/api/auth", authRequest2)
	var authResponse2 AuthResponse
	err = json.Unmarshal(authRecorder2.Body.Bytes(), &authResponse2)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}

	// Отправляем монеты от первого пользователя ко второму
	sendCoinRequest := SendCoinRequest{
		ToUser: "testuser2",
		Amount: 100,
	}
	token := authResponse1.Token
	body, _ := json.Marshal(sendCoinRequest)
	req := httptest.NewRequest(http.MethodPost, "/api/sendCoin", bytes.NewBuffer(body))
	req.Header.Set("Authorization", token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// Проверяем статус код
	assert.Equal(t, http.StatusOK, recorder.Code, "Ожидался код 200")

	// Проверяем сообщение об успешной отправке
	var responseMap map[string]string
	err = json.Unmarshal(recorder.Body.Bytes(), &responseMap)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}
	expectedMessage := "Coins sent successfully"
	assert.Equal(t, expectedMessage, responseMap["message"], "Неверное сообщение об успехе")
}

func TestSendCoins_HappyPath(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	router := NewRouter()

	// Создаем двух пользователей
	_, err := db.Exec("INSERT INTO users (username, password, coins) VALUES ($1, $2, $3)", "user1", "password1", 1000)
	if err != nil {
		t.Fatalf("Не удалось создать пользователя user1: %v", err)
	}
	_, err = db.Exec("INSERT INTO users (username, password, coins) VALUES ($1, $2, $3)", "user2", "password2", 1000)
	if err != nil {
		t.Fatalf("Не удалось создать пользователя user2: %v", err)
	}

	// Авторизуемся как user1
	authRequest := AuthRequest{
		Username: "user1",
		Password: "password1",
	}
	authRecorder := performRequest(router, "POST", "/api/auth", authRequest)
	var authResponse AuthResponse
	err = json.Unmarshal(authRecorder.Body.Bytes(), &authResponse)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}
	token := authResponse.Token

	// Проверяем баланс user1 до отправки монет
	var initialCoins int
	err = db.QueryRow("SELECT coins FROM users WHERE username = $1", "user1").Scan(&initialCoins)
	if err != nil || initialCoins != 1000 {
		t.Fatalf("Неверный начальный баланс user1: %v", err)
	}

	// Отправляем 100 монет от user1 к user2
	sendCoinRequest := SendCoinRequest{
		ToUser: "user2",
		Amount: 100,
	}
	body, _ := json.Marshal(sendCoinRequest)
	req := httptest.NewRequest(http.MethodPost, "/api/sendCoin", bytes.NewBuffer(body))
	req.Header.Set("Authorization", token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// Проверяем код ответа
	assert.Equal(t, http.StatusOK, recorder.Code, "Ожидался код 200")

	// Проверяем баланс user1 после отправки монет
	var updatedCoins int
	err = db.QueryRow("SELECT coins FROM users WHERE username = $1", "user1").Scan(&updatedCoins)
	if err != nil || updatedCoins != 900 {
		t.Fatalf("Неверный баланс user1 после отправки монет: %v", err)
	}

	// Проверяем баланс user2 после получения монет
	var updatedCoinsUser2 int
	err = db.QueryRow("SELECT coins FROM users WHERE username = $1", "user2").Scan(&updatedCoinsUser2)
	if err != nil || updatedCoinsUser2 != 1100 {
		t.Fatalf("Неверный баланс user2 после получения монет: %v", err)
	}
}

func TestSendCoins_NegativePath(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	router := NewRouter()

	// Создаем пользователя
	_, err := db.Exec("INSERT INTO users (username, password, coins) VALUES ($1, $2, $3)", "user1", "password1", 1000)
	if err != nil {
		t.Fatalf("Не удалось создать пользователя user1: %v", err)
	}

	// Авторизуемся как user1
	authRequest := AuthRequest{
		Username: "user1",
		Password: "password1",
	}
	authRecorder := performRequest(router, "POST", "/api/auth", authRequest)
	var authResponse AuthResponse
	err = json.Unmarshal(authRecorder.Body.Bytes(), &authResponse)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}
	token := authResponse.Token

	// Проверяем баланс user1 до отправки монет
	var initialCoins int
	err = db.QueryRow("SELECT coins FROM users WHERE username = $1", "user1").Scan(&initialCoins)
	if err != nil || initialCoins != 1000 {
		t.Fatalf("Неверный начальный баланс user1: %v", err)
	}

	// Попытка отправить 1001 монету
	sendCoinRequest := SendCoinRequest{
		ToUser: "user2",
		Amount: 1001,
	}
	body, _ := json.Marshal(sendCoinRequest)
	req := httptest.NewRequest(http.MethodPost, "/api/sendCoin", bytes.NewBuffer(body))
	req.Header.Set("Authorization", token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// Проверяем код ответа
	assert.Equal(t, http.StatusBadRequest, recorder.Code, "Ожидался код 400")

	// Проверяем баланс user1 после неудачной попытки отправки монет
	var updatedCoins int
	err = db.QueryRow("SELECT coins FROM users WHERE username = $1", "user1").Scan(&updatedCoins)
	if err != nil || updatedCoins != 1000 {
		t.Fatalf("Баланс user1 изменился после неудачной попытки отправки монет: %v", err)
	}
}

func TestBuyMerch_HappyPath(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	router := NewRouter()

	// Авторизуемся как user1
	authRequest := AuthRequest{
		Username: "user1",
		Password: "password1",
	}
	authRecorder := performRequest(router, "POST", "/api/auth", authRequest)
	var authResponse AuthResponse
	err := json.Unmarshal(authRecorder.Body.Bytes(), &authResponse)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}
	token := authResponse.Token

	// Проверяем баланс user1 до покупки
	var initialCoins int
	err = db.QueryRow("SELECT coins FROM users WHERE username = $1", "user1").Scan(&initialCoins)
	if err != nil || initialCoins != 1000 {
		t.Fatalf("Неверный начальный баланс user1: %v", err)
	}

	itemName := "hoody" // Предполагается, что hoody стоит 300 монет
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/buy/%s", itemName), nil)
	req.Header.Set("Authorization", token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// Проверяем код ответа
	assert.Equal(t, http.StatusOK, recorder.Code, "Ожидался код 200")

	// Проверяем баланс user1 после покупки
	var updatedCoins int
	err = db.QueryRow("SELECT coins FROM users WHERE username = $1", "user1").Scan(&updatedCoins)
	if err != nil || updatedCoins != 700 {
		t.Fatalf("Неверный баланс user1 после покупки мерча: %v", err)
	}
}

func TestBuyMerch_NotEnoughCoins(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	router := NewRouter()

	// Создаем пользователя
	_, err := db.Exec("INSERT INTO users (username, password, coins) VALUES ($1, $2, $3)", "user1", "password1", 499)
	if err != nil {
		t.Fatalf("Не удалось создать пользователя user1: %v", err)
	}

	// Авторизуемся как user1
	authRequest := AuthRequest{
		Username: "user1",
		Password: "password1",
	}
	authRecorder := performRequest(router, "POST", "/api/auth", authRequest)
	var authResponse AuthResponse
	err = json.Unmarshal(authRecorder.Body.Bytes(), &authResponse)
	if err != nil {
		t.Fatalf("Ошибка при парсинге JSON: %v", err)
	}
	token := authResponse.Token

	itemName := "pink-hoody"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/buy/%s", itemName), nil)
	req.Header.Set("Authorization", token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// Проверяем код ответа
	assert.Equal(t, http.StatusBadRequest, recorder.Code, "Ожидался код 400")

	// Проверяем баланс user1 после неудачной попытки покупки
	var updatedCoins int
	err = db.QueryRow("SELECT coins FROM users WHERE username = $1", "user1").Scan(&updatedCoins)
	if err != nil || updatedCoins != 499 {
		t.Fatalf("Баланс user1 изменился после неудачной попытки покупки мерча: %v", err)
	}
}

func TestBuyMerch_Unauthorized(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	router := NewRouter()

	// Попытка купить мерч без авторизации
	itemName := "hoody"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/buy/%s", itemName), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// Проверяем код ответа
	assert.Equal(t, http.StatusUnauthorized, recorder.Code, "Ожидался код 401")
}

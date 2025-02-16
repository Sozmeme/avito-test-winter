package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

var jwtKey = []byte("my_secret_key")

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

var tokenCache sync.Map // Кэш для хранения проверенных токенов

// Функция для проверки токена с использованием кэша
func validateToken(tokenString string) (*Claims, error) {
	// Проверяем, есть ли токен в кэше
	if claims, ok := tokenCache.Load(tokenString); ok {
		return claims.(*Claims), nil
	}

	// Если токена нет в кэше, проверяем его стандартным способом
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Сохраняем токен в кэше
	tokenCache.Store(tokenString, claims)

	return claims, nil
}

func ApiAuthPost(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	var userID int
	var storedPassword string
	err := db.QueryRow("SELECT id, password FROM users WHERE username = $1", req.Username).Scan(&userID, &storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			// Пользователь не существует, создаем нового
			_, err = db.Exec("INSERT INTO users (username, password, coins) VALUES ($1, $2, $3)", req.Username, req.Password, 1000)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Failed to create user")
				return
			}
		} else {
			respondWithError(w, http.StatusInternalServerError, "Database error")
			return
		}
	} else {
		// Пользователь существует, проверяем пароль
		if req.Password != storedPassword {
			respondWithError(w, http.StatusUnauthorized, "Invalid password")
			return
		}
	}

	// Генерация JWT токена
	expirationTime := time.Now().Add(5 * time.Minute)
	claims := &Claims{
		Username: req.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create token")
		return
	}

	respondWithJSON(w, http.StatusOK, AuthResponse{Token: tokenString})
}

var itemPrices = map[string]int{
	"t-shirt":    0,
	"cup":        0,
	"book":       0,
	"pen":        0,
	"powerbank":  0,
	"hoody":      0,
	"umbrella":   0,
	"socks":      0,
	"wallet":     0,
	"pink-hoody": 0,
}

func ApiBuyItemGet(w http.ResponseWriter, r *http.Request) {
	// Проверка токена
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		respondWithError(w, http.StatusUnauthorized, "Missing Authorization header")
		return
	}

	claims, err := validateToken(tokenString)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Получаем название товара из URL
	vars := mux.Vars(r)
	itemName := vars["item"]

	// Проверяем, что товар существует
	itemPrice, ok := itemPrices[itemName]
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Item not found")
		return
	}

	// Получаем ID и баланс пользователя
	var userID int
	var coins int
	err = db.QueryRow("SELECT id, coins FROM users WHERE username = $1", claims.Username).Scan(&userID, &coins)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Проверяем, хватает ли монет
	if coins < itemPrice {
		respondWithError(w, http.StatusBadRequest, "Not enough coins")
		return
	}

	// Начинаем транзакцию
	tx, err := db.Begin()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to start transaction")
		return
	}

	// Выполняем операции в отдельной горутине
	done := make(chan bool)
	go func() {
		// Списываем монеты с баланса пользователя
		_, err = tx.Exec("UPDATE users SET coins = coins - $1 WHERE id = $2", itemPrice, userID)
		if err != nil {
			tx.Rollback()
			respondWithError(w, http.StatusInternalServerError, "Failed to update balance")
			close(done)
			return
		}

		// Добавляем запись о покупке
		_, err = tx.Exec("INSERT INTO purchases (user_id, item_name) VALUES ($1, $2)", userID, itemName)
		if err != nil {
			tx.Rollback()
			respondWithError(w, http.StatusInternalServerError, "Failed to record purchase")
			close(done)
			return
		}

		// Завершаем транзакцию
		err = tx.Commit()
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to commit transaction")
			close(done)
			return
		}

		close(done)
	}()

	<-done // Ждём завершения операций
	// Возвращаем успешный ответ
	respondWithJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("You bought a %s!", itemName)})
}

func ApiInfoGet(w http.ResponseWriter, r *http.Request) {
	// Проверка токена
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		respondWithError(w, http.StatusUnauthorized, "Missing Authorization header")
		return
	}

	claims, err := validateToken(tokenString)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Получаем ID пользователя
	var userID int
	var coins int32
	err = db.QueryRow("SELECT id, coins FROM users WHERE username = $1", claims.Username).Scan(&userID, &coins)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Получаем инвентарь пользователя
	inventoryChan := make(chan []InfoResponseInventory, 1)
	go func() {
		var inventory []InfoResponseInventory
		rows, err := db.Query("SELECT item_name, COUNT(*) as quantity FROM purchases WHERE user_id = $1 GROUP BY item_name", userID)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to fetch inventory")
			close(inventoryChan)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var itemName string
			var quantity int32
			err := rows.Scan(&itemName, &quantity)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Failed to scan inventory")
				close(inventoryChan)
				return
			}
			inventory = append(inventory, InfoResponseInventory{
				Type_:    itemName,
				Quantity: quantity,
			})
		}
		inventoryChan <- inventory
	}()

	// Получаем историю полученных монет
	receivedChan := make(chan []InfoResponseCoinHistoryReceived, 1)
	go func() {
		var received []InfoResponseCoinHistoryReceived
		rows, err := db.Query(`
            SELECT u.username, t.amount 
            FROM transactions t 
            JOIN users u ON t.sender_id = u.id 
            WHERE t.receiver_id = $1
        `, userID)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to fetch received transactions")
			close(receivedChan)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var fromUser string
			var amount int32
			err := rows.Scan(&fromUser, &amount)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Failed to scan received transaction")
				close(receivedChan)
				return
			}
			received = append(received, InfoResponseCoinHistoryReceived{
				FromUser: fromUser,
				Amount:   amount,
			})
		}
		receivedChan <- received
	}()

	// Получаем историю отправленных монет
	sentChan := make(chan []InfoResponseCoinHistorySent, 1)
	go func() {
		var sent []InfoResponseCoinHistorySent
		rows, err := db.Query(`
            SELECT u.username, t.amount 
            FROM transactions t 
            JOIN users u ON t.receiver_id = u.id 
            WHERE t.sender_id = $1
        `, userID)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to fetch sent transactions")
			close(sentChan)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var toUser string
			var amount int32
			err := rows.Scan(&toUser, &amount)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Failed to scan sent transaction")
				close(sentChan)
				return
			}
			sent = append(sent, InfoResponseCoinHistorySent{
				ToUser: toUser,
				Amount: amount,
			})
		}
		sentChan <- sent
	}()

	inventory := <-inventoryChan
	received := <-receivedChan
	sent := <-sentChan

	response := InfoResponse{
		Coins:     coins,
		Inventory: inventory,
		CoinHistory: &InfoResponseCoinHistory{
			Received: received,
			Sent:     sent,
		},
	}

	respondWithJSON(w, http.StatusOK, response)
}

func ApiSendCoinPost(w http.ResponseWriter, r *http.Request) {
	// Проверка токена
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		respondWithError(w, http.StatusUnauthorized, "Missing Authorization header")
		return
	}

	claims, err := validateToken(tokenString)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Получаем данные из запроса
	var req SendCoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Проверяем, что сумма положительная
	if req.Amount < 0 {
		respondWithError(w, http.StatusBadRequest, "Amount must be positive")
		return
	}

	// Получаем ID и баланс отправителя
	var senderID int
	var senderCoins int32
	err = db.QueryRow("SELECT id, coins FROM users WHERE username = $1", claims.Username).Scan(&senderID, &senderCoins)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Проверяем, что отправитель не пытается отправить монеты сам себе
	if claims.Username == req.ToUser {
		respondWithError(w, http.StatusBadRequest, "Cannot send coins to yourself")
		return
	}

	// Проверяем, хватает ли монет у отправителя
	if senderCoins < req.Amount {
		respondWithError(w, http.StatusBadRequest, "Not enough coins")
		return
	}

	// Получаем ID получателя
	var receiverID int
	err = db.QueryRow("SELECT id FROM users WHERE username = $1", req.ToUser).Scan(&receiverID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusBadRequest, "Receiver not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Начинаем транзакцию
	tx, err := db.Begin()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to start transaction")
		return
	}

	done := make(chan bool)
	go func() {
		// Списываем монеты с отправителя
		_, err = tx.Exec("UPDATE users SET coins = coins - $1 WHERE id = $2", req.Amount, senderID)
		if err != nil {
			tx.Rollback()
			respondWithError(w, http.StatusInternalServerError, "Failed to update sender balance")
			close(done)
			return
		}

		// Добавляем монеты получателю
		_, err = tx.Exec("UPDATE users SET coins = coins + $1 WHERE id = $2", req.Amount, receiverID)
		if err != nil {
			tx.Rollback()
			respondWithError(w, http.StatusInternalServerError, "Failed to update receiver balance")
			close(done)
			return
		}

		// Записываем транзакцию
		_, err = tx.Exec("INSERT INTO transactions (sender_id, receiver_id, amount) VALUES ($1, $2, $3)", senderID, receiverID, req.Amount)
		if err != nil {
			tx.Rollback()
			respondWithError(w, http.StatusInternalServerError, "Failed to record transaction")
			close(done)
			return
		}

		// Завершаем транзакцию
		err = tx.Commit()
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to commit transaction")
			close(done)
			return
		}

		close(done)
	}()

	<-done

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Coins sent successfully"})
}

func respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(payload)
}

func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	respondWithJSON(w, statusCode, ErrorResponse{Errors: message})
}

var db *sql.DB

func initdb() (*sql.DB, error) {
	time.Sleep(1 * time.Second)
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func main() {
	var err error
	db, err = initdb()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	log.Println("Successfully connected to the database")

	log.Printf("Server started")

	router := NewRouter()

	log.Fatal(http.ListenAndServe(":8080", router))
}

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc

		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}

	return router
}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World!")
}

var routes = Routes{
	Route{
		"Index",
		"GET",
		"/",
		Index,
	},

	Route{
		"ApiAuthPost",
		strings.ToUpper("Post"),
		"/api/auth",
		ApiAuthPost,
	},

	Route{
		"ApiBuyItemGet",
		strings.ToUpper("Get"),
		"/api/buy/{item}",
		ApiBuyItemGet,
	},

	Route{
		"ApiInfoGet",
		strings.ToUpper("Get"),
		"/api/info",
		ApiInfoGet,
	},

	Route{
		"ApiSendCoinPost",
		strings.ToUpper("Post"),
		"/api/sendCoin",
		ApiSendCoinPost,
	},
}

type AuthRequest struct {

	// Имя пользователя для аутентификации.
	Username string `json:"username"`

	// Пароль для аутентификации.
	Password string `json:"password"`
}

type AuthResponse struct {

	// JWT-токен для доступа к защищенным ресурсам.
	Token string `json:"token,omitempty"`
}

type ErrorResponse struct {

	// Сообщение об ошибке, описывающее проблему.
	Errors string `json:"errors,omitempty"`
}

type InfoResponseCoinHistoryReceived struct {

	// Имя пользователя, который отправил монеты.
	FromUser string `json:"fromUser,omitempty"`

	// Количество полученных монет.
	Amount int32 `json:"amount,omitempty"`
}

type InfoResponseCoinHistorySent struct {

	// Имя пользователя, которому отправлены монеты.
	ToUser string `json:"toUser,omitempty"`

	// Количество отправленных монет.
	Amount int32 `json:"amount,omitempty"`
}

type InfoResponseCoinHistory struct {
	Received []InfoResponseCoinHistoryReceived `json:"received,omitempty"`

	Sent []InfoResponseCoinHistorySent `json:"sent,omitempty"`
}

type InfoResponseInventory struct {

	// Тип предмета.
	Type_ string `json:"type,omitempty"`

	// Количество предметов.
	Quantity int32 `json:"quantity,omitempty"`
}

type InfoResponse struct {

	// Количество доступных монет.
	Coins int32 `json:"coins,omitempty"`

	Inventory []InfoResponseInventory `json:"inventory,omitempty"`

	CoinHistory *InfoResponseCoinHistory `json:"coinHistory,omitempty"`
}

type SendCoinRequest struct {

	// Имя пользователя, которому нужно отправить монеты.
	ToUser string `json:"toUser"`

	// Количество монет, которые необходимо отправить.
	Amount int32 `json:"amount"`
}

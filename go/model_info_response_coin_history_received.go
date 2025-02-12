package swagger

type InfoResponseCoinHistoryReceived struct {

	// Имя пользователя, который отправил монеты.
	FromUser string `json:"fromUser,omitempty"`

	// Количество полученных монет.
	Amount int32 `json:"amount,omitempty"`
}

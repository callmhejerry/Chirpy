package responses

import "time"

type ChirpResponseModel struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserId    string    `json:"user_id"`
	Body      string    `json:"body"`
}

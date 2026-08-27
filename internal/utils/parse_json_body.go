package utils

import (
	"encoding/json"
	"io"
)

func ParseRequestBody[T any](requestBody io.Reader) (T, error) {
	var body T
	jsonDecoder := json.NewDecoder(requestBody)

	if err := jsonDecoder.Decode(&body); err != nil {
		return body, err
	}
	return body, nil
}

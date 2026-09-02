package auth

import (
	"net/http"
	"testing"
)

type TestCase struct {
	token         string
	expectedToken string
	shouldFail    bool
}

func TestGetBearerToken(t *testing.T) {
	testCases := []TestCase{
		{
			token:         "Bearer mytoken",
			expectedToken: "mytoken",
			shouldFail:    false,
		},
		{
			token:         "Bearer   mytoken",
			expectedToken: "",
			shouldFail:    true,
		},
		{
			token:         "Bearer",
			expectedToken: "",
			shouldFail:    true,
		},
		{
			token:         "InvalidToken",
			expectedToken: "",
			shouldFail:    true,
		},
	}

	for _, testCase := range testCases {
		actualToken, err := GetBearerToken(http.Header{"Authorization": []string{testCase.token}})

		if testCase.shouldFail {
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		} else {
			if actualToken != testCase.expectedToken {
				t.Errorf("Expected token %s, got %s", testCase.expectedToken, actualToken)
			}
		}

	}
}

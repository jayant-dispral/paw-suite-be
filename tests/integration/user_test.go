package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserHandlers(t *testing.T) {
	// Clean the user collection before this test suite runs
	err := testDB.Collection("users").Drop(context.Background())
	require.NoError(t, err, "Failed to drop users collection for user tests")

	var userToken string
	userEmail := "user-test@example.com"
	userPassword := "password123"

	// --- SETUP: Create a user and get a token ---
	t.Run("SETUP - Register and Login User", func(t *testing.T) {
		// Register
		regBody, _ := json.Marshal(map[string]string{"email": userEmail, "password": userPassword})
		regReq := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(regBody))
		regReq.Header.Set("Content-Type", "application/json")
		regRR := httptest.NewRecorder()
		testRouter.ServeHTTP(regRR, regReq)
		require.Equal(t, http.StatusCreated, regRR.Code, "Setup: Failed to register user")

		// Login
		loginBody, _ := json.Marshal(map[string]string{"email": userEmail, "password": userPassword})
		loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginBody))
		loginReq.Header.Set("Content-Type", "application/json")
		loginRR := httptest.NewRecorder()
		testRouter.ServeHTTP(loginRR, loginReq)
		require.Equal(t, http.StatusOK, loginRR.Code, "Setup: Failed to login user")

		var respBody map[string]interface{}
		err := json.Unmarshal(loginRR.Body.Bytes(), &respBody)
		require.NoError(t, err, "Setup: Failed to unmarshal login response")
		token, ok := respBody["token"].(string)
		require.True(t, ok, "Setup: Token not found in login response")
		userToken = token
	})

	require.NotEmpty(t, userToken, "Setup: User token should not be empty")

	// --- GET /me ---
	t.Run("GET /users/me - Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()

		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var respBody map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &respBody)
		require.NoError(t, err)
		data, ok := respBody["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, userEmail, data["email"])
	})

	t.Run("GET /users/me - No Auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	// --- POST /me (Update) ---
	t.Run("POST /users/me - Success", func(t *testing.T) {
		newName := "Updated User Name"
		updateData := domain.UpdateUserStruct{FullName: &newName}
		body, _ := json.Marshal(updateData)

		req := httptest.NewRequest("POST", "/api/v1/users/me", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()

		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Verify the response contains the updated name
		var respBody map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &respBody)
		require.NoError(t, err)
		data, ok := respBody["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, newName, data["full_name"])
		assert.Equal(t, userEmail, data["email"]) // Email should not change
	})

	t.Run("POST /users/me - Invalid Body", func(t *testing.T) {
		// Send a malformed JSON
		body := []byte(`{"full_name": "test"`)
		req := httptest.NewRequest("POST", "/api/v1/users/me", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()

		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
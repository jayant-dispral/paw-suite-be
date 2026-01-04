package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthHandlers(t *testing.T) {
	// Clean the user collection before this test suite runs
	err := testDB.Collection("users").Drop(context.Background())
	require.NoError(t, err, "Failed to drop users collection for auth tests")

	// --- REGISTRATION ---
	t.Run("POST /auth/register - Success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    "auth-test@example.com",
			"password": "password123",
		})
		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var respBody map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &respBody)
		require.NoError(t, err)
		assert.NotEmpty(t, respBody["user_id"])
	})

	t.Run("POST /auth/register - Conflict (Email Exists)", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    "auth-test@example.com", // Same email as above
			"password": "anotherpassword",
		})
		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("POST /auth/register - Invalid Input", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email": "not-an-email",
			// Missing password
		})
		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	// --- LOGIN ---
	t.Run("POST /auth/login - Success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    "auth-test@example.com",
			"password": "password123",
		})
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var respBody map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &respBody)
		require.NoError(t, err)
		assert.NotEmpty(t, respBody["token"])
		assert.NotEmpty(t, respBody["user_id"])
	})

	t.Run("POST /auth/login - Invalid Credentials (Wrong Password)", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    "auth-test@example.com",
			"password": "wrong-password",
		})
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("POST /auth/login - User Not Found", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    "not-found@example.com",
			"password": "password123",
		})
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}
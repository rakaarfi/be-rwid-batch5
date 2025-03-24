package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegister(t *testing.T) {
	app := CreateTestApp()

	uniqueUsername := fmt.Sprintf("testuser_%d", time.Now().UnixNano())
	reqBody := map[string]string{
		"username": uniqueUsername,
		"email":    uniqueUsername + "@example.com",
		"password": "secret123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Register request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status %d or %d but got %d", http.StatusOK, http.StatusCreated, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Error decoding register response: %v", err)
	}

	if result["data"] == nil {
		t.Errorf("Expected data field in response")
	}
}

func TestLogin(t *testing.T) {
	app := CreateTestApp()

	// Pastikan user yang akan di-login sudah terdaftar.
	// Jika perlu, register terlebih dahulu
	{
		reqBody := map[string]string{
			"username": "testlogin",
			"email":    "testlogin@example.com",
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		_, _ = app.Test(req)
		// Abaikan error karena user bisa saja sudah ada
		time.Sleep(100 * time.Millisecond) // sedikit delay untuk memastikan user tersimpan
	}

	// Siapkan data login
	loginBody := map[string]string{
		"username": "testlogin",
		"password": "password123",
	}
	body, _ := json.Marshal(loginBody)

	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Error decoding login response: %v", err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in login response")
	}
	if data["token"] == nil {
		t.Errorf("Expected token in login response")
	}
}

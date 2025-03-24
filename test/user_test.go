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

func TestGetAllUsers(t *testing.T) {
	app := CreateTestApp()

	// Buat admin user dan login untuk mendapatkan token
	adminToken, _, _ := CreateTestAdmin(app, t)

	// Lakukan request GET /users dengan header Authorization
	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Error in getAllUsers request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Error decoding getAllUsers response: %v", err)
	}
	data, ok := result["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Errorf("Expected non-empty data field in response")
	}
}

func TestGetUser(t *testing.T) {
	app := CreateTestApp()

	// Buat admin user dan login
	adminToken, adminID, adminUsername := CreateTestAdmin(app, t)

	// GET user dengan admin token (admin dapat mengakses user lain)
	req := httptest.NewRequest("GET", fmt.Sprintf("/users/%d", adminID), nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Error in getUser request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Error decoding getUser response: %v", err)
	}
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in getUser response")
	}
	if data["username"] != adminUsername {
		t.Errorf("Expected username %s but got %v", adminUsername, data["username"])
	}
}

func TestUpdateUser(t *testing.T) {
	app := CreateTestApp()

	// Buat user reguler melalui register
	uniqueUser := fmt.Sprintf("user_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "userpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := app.Test(regReq)
	if err != nil {
		t.Fatalf("Error in register request for updateUser test: %v", err)
	}
	defer regResp.Body.Close()

	var regResult map[string]interface{}
	if err := json.NewDecoder(regResp.Body).Decode(&regResult); err != nil {
		t.Fatalf("Error decoding register response: %v", err)
	}
	data, ok := regResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in register response")
	}
	userIDFloat, ok := data["id"].(float64)
	if !ok {
		t.Fatalf("Expected user ID in register response")
	}
	userID := int(userIDFloat)

	// Login user untuk mendapatkan token
	loginBody := map[string]string{
		"username": uniqueUser,
		"password": "userpass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Error in login request for updateUser test: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Fatalf("Error decoding login response: %v", err)
	}
	loginData, ok := loginResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in login response")
	}
	token, ok := loginData["token"].(string)
	if !ok || token == "" {
		t.Fatalf("Expected valid token for updateUser test")
	}

	// Lakukan update user, misalnya ubah username
	newUsername := uniqueUser + "_updated"
	updateBody := map[string]string{
		"username": newUsername,
		"password": "newpass123",
		// kita bisa update email atau password jika diinginkan
	}
	updateJSON, _ := json.Marshal(updateBody)
	updateReq := httptest.NewRequest("PUT", fmt.Sprintf("/users/%d", userID), bytes.NewReader(updateJSON))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("Error in updateUser request: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, updateResp.StatusCode)
	}

	// Ambil user kembali untuk memastikan update
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/users/%d", userID), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("Error in getUser after update request: %v", err)
	}
	defer getResp.Body.Close()
	var getResult map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&getResult); err != nil {
		t.Fatalf("Error decoding getUser after update response: %v", err)
	}
	getData, ok := getResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in getUser after update response")
	}
	if getData["username"] != newUsername {
		t.Errorf("Expected updated username %s but got %v", newUsername, getData["username"])
	}
}

func TestDeleteUser(t *testing.T) {
	app := CreateTestApp()

	// Buat user reguler melalui register
	uniqueUser := fmt.Sprintf("user_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "deletepass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := app.Test(regReq)
	if err != nil {
		t.Fatalf("Error in register request for deleteUser test: %v", err)
	}
	defer regResp.Body.Close()
	var regResult map[string]interface{}
	if err := json.NewDecoder(regResp.Body).Decode(&regResult); err != nil {
		t.Fatalf("Error decoding register response: %v", err)
	}
	data, ok := regResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in register response")
	}
	userIDFloat, ok := data["id"].(float64)
	if !ok {
		t.Fatalf("Expected user ID in register response")
	}
	userID := int(userIDFloat)

	// Login user untuk mendapatkan token
	loginBody := map[string]string{
		"username": uniqueUser,
		"password": "deletepass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Error in login request for deleteUser test: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Fatalf("Error decoding login response: %v", err)
	}
	loginData, ok := loginResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in login response")
	}
	token, ok := loginData["token"].(string)
	if !ok || token == "" {
		t.Fatalf("Expected valid token for deleteUser test")
	}

	// Lakukan request DELETE /users/:id
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/users/%d", userID), nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	if err != nil {
		t.Fatalf("Error in deleteUser request: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, delResp.StatusCode)
	}

	// Coba GET user tersebut, harus menghasilkan error 404
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/users/%d", userID), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("Error in getUser after delete request: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status %d for deleted user but got %d", http.StatusNotFound, getResp.StatusCode)
	}
}

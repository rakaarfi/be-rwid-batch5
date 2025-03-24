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

// TestCreateTask: Uji pembuatan task baru
func TestCreateTask(t *testing.T) {
	app := CreateTestApp()

	// Register dan login user untuk task
	uniqueUser := fmt.Sprintf("taskuser_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "taskpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	_, err := app.Test(regReq)
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	// Login
	loginBody := map[string]string{
		"username": uniqueUser,
		"password": "taskpass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Fatalf("Error decoding login response: %v", err)
	}
	token := loginResult["data"].(map[string]interface{})["token"].(string)
	if token == "" {
		t.Fatalf("Expected valid token")
	}

	// Create Task
	taskBody := map[string]string{
		"title":         "Test Task",
		"description":   "Task description",
		"status":        "pending",
		"security_code": "12345",
	}
	taskJSON, _ := json.Marshal(taskBody)
	taskReq := httptest.NewRequest("POST", "/tasks", bytes.NewReader(taskJSON))
	taskReq.Header.Set("Content-Type", "application/json")
	taskReq.Header.Set("Authorization", "Bearer "+token)
	taskResp, err := app.Test(taskReq)
	if err != nil {
		t.Fatalf("CreateTask error: %v", err)
	}
	defer taskResp.Body.Close()

	if taskResp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", taskResp.StatusCode)
	}
	var taskResult map[string]interface{}
	if err := json.NewDecoder(taskResp.Body).Decode(&taskResult); err != nil {
		t.Fatalf("Error decoding createTask response: %v", err)
	}
	if taskResult["id"] == nil {
		t.Errorf("Expected task id in response")
	}
}

// TestListTasks: Uji endpoint list tasks
func TestListTasks(t *testing.T) {
	app := CreateTestApp()

	uniqueUser := fmt.Sprintf("listuser_%d", time.Now().UnixNano())
	// Register & login
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "listpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(regReq)
	time.Sleep(100 * time.Millisecond) // pastikan data tersimpan

	loginBody := map[string]string{
		"username": uniqueUser,
		"password": "listpass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	token := loginResult["data"].(map[string]interface{})["token"].(string)

	// Buat satu task terlebih dahulu
	taskBody := map[string]string{
		"title":         "List Task",
		"description":   "List description",
		"status":        "pending",
		"security_code": "abcde",
	}
	taskJSON, _ := json.Marshal(taskBody)
	taskReq := httptest.NewRequest("POST", "/tasks", bytes.NewReader(taskJSON))
	taskReq.Header.Set("Content-Type", "application/json")
	taskReq.Header.Set("Authorization", "Bearer "+token)
	_, _ = app.Test(taskReq)

	// Panggil endpoint list tasks
	listReq := httptest.NewRequest("GET", "/tasks", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("ListTasks error: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for list tasks, got %d", listResp.StatusCode)
	}
	var listResult map[string]interface{}
	if err := json.NewDecoder(listResp.Body).Decode(&listResult); err != nil {
		t.Fatalf("Error decoding listTasks response: %v", err)
	}
	tasksData, ok := listResult["data"].([]interface{})
	if !ok || len(tasksData) == 0 {
		t.Errorf("Expected non-empty tasks data")
	}
}

// TestGetTask: Uji endpoint ambil task berdasarkan ID
func TestGetTask(t *testing.T) {
	app := CreateTestApp()

	uniqueUser := fmt.Sprintf("gettaskuser_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "gettaskpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(regReq)

	loginBody := map[string]string{"username": uniqueUser, "password": "gettaskpass"}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	token := loginResult["data"].(map[string]interface{})["token"].(string)

	// Buat task
	taskBody := map[string]string{
		"title":         "Get Task",
		"description":   "Get Task description",
		"status":        "pending",
		"security_code": "67890",
	}
	taskJSON, _ := json.Marshal(taskBody)
	taskReq := httptest.NewRequest("POST", "/tasks", bytes.NewReader(taskJSON))
	taskReq.Header.Set("Content-Type", "application/json")
	taskReq.Header.Set("Authorization", "Bearer "+token)
	taskResp, err := app.Test(taskReq)
	if err != nil {
		t.Fatalf("CreateTask error: %v", err)
	}
	defer taskResp.Body.Close()
	var taskResult map[string]interface{}
	json.NewDecoder(taskResp.Body).Decode(&taskResult)
	taskID := int(taskResult["id"].(float64))

	// Ambil task dengan GET /tasks/:id
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/tasks/%d", taskID), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GetTask error: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for get task, got %d", getResp.StatusCode)
	}
	var getResult map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&getResult)
	getData, ok := getResult["data"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected data field in get task response")
	}
	if getData["title"] != "Get Task" {
		t.Errorf("Expected title 'Get Task' but got %v", getData["title"])
	}
}

// TestUpdateTask: Uji endpoint update task
func TestUpdateTask(t *testing.T) {
	app := CreateTestApp()

	uniqueUser := fmt.Sprintf("updatetaskuser_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "updatetaskpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(regReq)

	loginBody := map[string]string{"username": uniqueUser, "password": "updatetaskpass"}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	token := loginResult["data"].(map[string]interface{})["token"].(string)

	// Buat task
	taskBody := map[string]string{
		"title":         "Old Task Title",
		"description":   "Old description",
		"status":        "pending",
		"security_code": "oldcode",
	}
	taskJSON, _ := json.Marshal(taskBody)
	taskReq := httptest.NewRequest("POST", "/tasks", bytes.NewReader(taskJSON))
	taskReq.Header.Set("Content-Type", "application/json")
	taskReq.Header.Set("Authorization", "Bearer "+token)
	taskResp, err := app.Test(taskReq)
	if err != nil {
		t.Fatalf("CreateTask error: %v", err)
	}
	defer taskResp.Body.Close()
	var taskResult map[string]interface{}
	json.NewDecoder(taskResp.Body).Decode(&taskResult)
	taskID := int(taskResult["id"].(float64))

	// Update task: ubah title dan security_code
	updateBody := map[string]string{
		"title":         "Updated Task Title",
		"security_code": "newsecret",
	}
	updateJSON, _ := json.Marshal(updateBody)
	updateReq := httptest.NewRequest("PUT", fmt.Sprintf("/tasks/%d", taskID), bytes.NewReader(updateJSON))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("UpdateTask error: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for update task, got %d", updateResp.StatusCode)
	}

	// Ambil task kembali untuk verifikasi
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/tasks/%d", taskID), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GetTask after update error: %v", err)
	}
	defer getResp.Body.Close()
	var getResult map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&getResult)
	getData, ok := getResult["data"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected data field in get task response")
	}
	if getData["title"] != "Updated Task Title" {
		t.Errorf("Expected updated title 'Updated Task Title' but got %v", getData["title"])
	}
	// Pastikan security_code sudah didekripsi dengan benar
	if getData["security_code"] != "newsecret" {
		t.Errorf("Expected decrypted security_code 'newsecret' but got %v", getData["security_code"])
	}
}

// TestDeleteTask: Uji endpoint hapus task
func TestDeleteTask(t *testing.T) {
	app := CreateTestApp()

	uniqueUser := fmt.Sprintf("deletetaskuser_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "deletetaskpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(regReq)

	loginBody := map[string]string{"username": uniqueUser, "password": "deletetaskpass"}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	token := loginResult["data"].(map[string]interface{})["token"].(string)

	// Buat task
	taskBody := map[string]string{
		"title":         "Task to Delete",
		"description":   "This task will be deleted",
		"status":        "pending",
		"security_code": "tobedeleted",
	}
	taskJSON, _ := json.Marshal(taskBody)
	taskReq := httptest.NewRequest("POST", "/tasks", bytes.NewReader(taskJSON))
	taskReq.Header.Set("Content-Type", "application/json")
	taskReq.Header.Set("Authorization", "Bearer "+token)
	taskResp, err := app.Test(taskReq)
	if err != nil {
		t.Fatalf("CreateTask error: %v", err)
	}
	defer taskResp.Body.Close()
	var taskResult map[string]interface{}
	json.NewDecoder(taskResp.Body).Decode(&taskResult)
	taskID := int(taskResult["id"].(float64))

	// Hapus task
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/tasks/%d", taskID), nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	if err != nil {
		t.Fatalf("DeleteTask error: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for delete task, got %d", delResp.StatusCode)
	}

	// Pastikan task sudah tidak ada (GET harus mengembalikan 404)
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/tasks/%d", taskID), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GetTask after delete error: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for deleted task, got %d", getResp.StatusCode)
	}
}

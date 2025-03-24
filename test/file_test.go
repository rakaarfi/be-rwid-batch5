package test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

func TestUploadProfilePicture(t *testing.T) {
	app := CreateTestApp()

	// Pertama, register dan login agar mendapatkan token
	{
		reqBody := map[string]string{
			"username": "uploaduser",
			"email":    "uploaduser@example.com",
			"password": "uploadpass",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		_, _ = app.Test(req)
		time.Sleep(100 * time.Millisecond)
	}

	// Lakukan login
	loginBody := map[string]string{
		"username": "uploaduser",
		"password": "uploadpass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	defer loginResp.Body.Close()

	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Fatalf("Error decoding login response: %v", err)
	}
	data, ok := loginResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in login response")
	}
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatalf("Expected valid token")
	}

	// Sekarang, buat request multipart untuk upload profile picture
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// Buat header untuk file dengan Content-Type yang valid
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="profile_picture"; filename="testpic.png"`)
	h.Set("Content-Type", "image/png") // Pastikan header content-type diset
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("Error creating form part: %v", err)
	}
	// Tulis data dummy untuk file (contoh: PNG signature)
	dummyData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	_, err = part.Write(dummyData)
	if err != nil {
		t.Fatalf("Error writing dummy file data: %v", err)
	}

	writer.Close()

	uploadReq := httptest.NewRequest("POST", "/upload/profile_picture", &b)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+token)

	uploadResp, err := app.Test(uploadReq)
	if err != nil {
		t.Fatalf("Upload profile picture request failed: %v", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, uploadResp.StatusCode)
	}

	uploadRespBody, err := io.ReadAll(uploadResp.Body)
	if err != nil {
		t.Fatalf("Error reading upload response: %v", err)
	}
	// Ubah response menjadi string untuk pengecekan sederhana
	respStr := string(uploadRespBody)
	if !strings.Contains(respStr, "Profile picture uploaded successfully") {
		t.Errorf("Unexpected upload response: %s", respStr)
	}
}

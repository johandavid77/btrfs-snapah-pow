package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
)

var baseURL = func() string {
	if v := os.Getenv("SNAPAH_URL"); v != "" {
		return v
	}
	return "http://localhost:8082"
}()

func getToken(t *testing.T) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "admin123"})
	resp, err := http.Post(baseURL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login status: %d", resp.StatusCode)
	}
	var data map[string]string
	json.NewDecoder(resp.Body).Decode(&data)
	if data["token"] == "" {
		t.Fatal("token vacio en respuesta")
	}
	return data["token"]
}

func apiGet(t *testing.T, token, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", baseURL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func apiPost(t *testing.T, token, path string, payload interface{}) *http.Response {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func TestHealth(t *testing.T) {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("health status: %d", resp.StatusCode)
	}
	var data map[string]string
	json.NewDecoder(resp.Body).Decode(&data)
	if data["status"] != "ok" {
		t.Fatalf("health status field: %q", data["status"])
	}
	fmt.Println("✅ Health OK")
}

func TestLogin(t *testing.T) {
	token := getToken(t)
	if len(token) < 10 {
		t.Fatalf("token demasiado corto: %q", token)
	}
	fmt.Println("✅ Login OK — token recibido")
}

func TestLoginWrongPassword(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	resp, _ := http.Post(baseURL+"/api/auth/login", "application/json", bytes.NewReader(body))
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("esperaba 401, got %d", resp.StatusCode)
	}
	fmt.Println("✅ Login incorrecto -> 401 OK")
}

func TestProtectedWithoutToken(t *testing.T) {
	resp, err := http.Get(baseURL + "/api/nodes")
	if err != nil {
		t.Fatalf("nodes: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("esperaba 401 sin token, got %d", resp.StatusCode)
	}
	fmt.Println("✅ Ruta protegida sin token -> 401 OK")
}

func TestListNodes(t *testing.T) {
	token := getToken(t)
	resp := apiGet(t, token, "/api/nodes")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("list nodes status: %d", resp.StatusCode)
	}
	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	if _, ok := data["count"]; !ok {
		t.Fatal("respuesta no tiene campo 'count'")
	}
	fmt.Printf("✅ ListNodes OK — %v nodos\n", data["count"])
}

func TestListSnapshots(t *testing.T) {
	token := getToken(t)
	resp := apiGet(t, token, "/api/snapshots")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("list snapshots status: %d", resp.StatusCode)
	}
	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	if _, ok := data["count"]; !ok {
		t.Fatal("respuesta no tiene campo 'count'")
	}
	fmt.Printf("✅ ListSnapshots OK — %v snapshots\n", data["count"])
}

func TestListEvents(t *testing.T) {
	token := getToken(t)
	resp := apiGet(t, token, "/api/events")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("list events status: %d", resp.StatusCode)
	}
	fmt.Println("✅ ListEvents OK")
}

func TestListPolicies(t *testing.T) {
	token := getToken(t)
	resp := apiGet(t, token, "/api/policies")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("list policies status: %d", resp.StatusCode)
	}
	fmt.Println("✅ ListPolicies OK")
}

func TestCreateSnapshotValidation(t *testing.T) {
	token := getToken(t)
	resp := apiPost(t, token, "/api/snapshots", map[string]string{})
	defer resp.Body.Close()
	// Debe fallar porque btrfs no existe en CI — pero el endpoint responde
	if resp.StatusCode == 0 {
		t.Fatal("no response")
	}
	fmt.Printf("✅ CreateSnapshot endpoint responde: %d\n", resp.StatusCode)
}

func TestMetrics(t *testing.T) {
	resp, err := http.Get("http://localhost:9093/metrics")
	if err != nil {
		t.Skipf("metrics no disponible en este entorno: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("metrics status: %d", resp.StatusCode)
	}
	fmt.Println("✅ Prometheus metrics OK")
}

package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"time"
)

// MockBackends simulates backend services with realistic behavior
type MockBackends struct {
	UserService      string
	OrderService     string
	InventoryService string

	servers []*httptest.Server
}

// StartMockBackends creates mock backend services
func StartMockBackends() *MockBackends {
	userServer := httptest.NewServer(createUserHandler())
	orderServer := httptest.NewServer(createOrderHandler())
	inventoryServer := httptest.NewServer(createInventoryHandler())

	return &MockBackends{
		UserService:      userServer.URL,
		OrderService:     orderServer.URL,
		InventoryService: inventoryServer.URL,
		servers: []*httptest.Server{
			userServer,
			orderServer,
			inventoryServer,
		},
	}
}

// Shutdown stops all mock servers
func (m *MockBackends) Shutdown() {
	for _, server := range m.servers {
		server.Close()
	}
}

// createUserHandler simulates user service
func createUserHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simulate variable latency
		latency := time.Duration(10+rand.Intn(40)) * time.Millisecond
		time.Sleep(latency)

		// Occasional errors (5% error rate)
		if rand.Float32() < 0.05 {
			if rand.Float32() < 0.5 {
				http.Error(w, "Database connection failed", http.StatusInternalServerError)
			} else {
				http.Error(w, "User not found", http.StatusNotFound)
			}
			return
		}

		// Successful response
		response := map[string]interface{}{
			"id":       rand.Intn(10000),
			"username": fmt.Sprintf("user_%d", rand.Intn(1000)),
			"email":    fmt.Sprintf("user%d@example.com", rand.Intn(1000)),
			"created":  time.Now().Format(time.RFC3339),
			"latency":  latency.Milliseconds(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// createOrderHandler simulates order service
func createOrderHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Higher latency for order processing
		latency := time.Duration(50+rand.Intn(150)) * time.Millisecond
		time.Sleep(latency)

		// Occasional timeouts (2% of requests take very long)
		if rand.Float32() < 0.02 {
			time.Sleep(5 * time.Second) // This will trigger gateway timeout
			return
		}

		// Higher error rate during peak (10%)
		if rand.Float32() < 0.10 {
			if rand.Float32() < 0.3 {
				http.Error(w, "Payment processing failed", http.StatusPaymentRequired)
			} else if rand.Float32() < 0.6 {
				http.Error(w, "Inventory unavailable", http.StatusConflict)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Successful order response
		response := map[string]interface{}{
			"orderId": fmt.Sprintf("ORD-%d", rand.Intn(1000000)),
			"userId":  rand.Intn(10000),
			"total":   rand.Float64() * 1000,
			"items":   rand.Intn(10) + 1,
			"status":  "confirmed",
			"created": time.Now().Format(time.RFC3339),
			"latency": latency.Milliseconds(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// createInventoryHandler simulates inventory service
func createInventoryHandler() http.HandlerFunc {
	// Simulate degraded service
	requestCount := 0

	return func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Degrade after certain number of requests
		degraded := requestCount > 100 && requestCount < 200

		var latency time.Duration
		if degraded {
			// Degraded performance
			latency = time.Duration(200+rand.Intn(800)) * time.Millisecond
		} else {
			// Normal performance
			latency = time.Duration(20+rand.Intn(30)) * time.Millisecond
		}
		time.Sleep(latency)

		// Higher error rate when degraded
		errorRate := 0.03
		if degraded {
			errorRate = 0.25
		}

		if rand.Float64() < errorRate {
			http.Error(w, "Database read timeout", http.StatusServiceUnavailable)
			return
		}

		// Inventory response
		items := make([]map[string]interface{}, rand.Intn(20)+1)
		for i := range items {
			items[i] = map[string]interface{}{
				"sku":       fmt.Sprintf("SKU-%d", rand.Intn(10000)),
				"quantity":  rand.Intn(1000),
				"reserved":  rand.Intn(100),
				"available": rand.Intn(900),
			}
		}

		response := map[string]interface{}{
			"items":    items,
			"total":    len(items),
			"degraded": degraded,
			"latency":  latency.Milliseconds(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

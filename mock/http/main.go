package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// HTTP Tool Mock Server

func main() {
	// Order endpoint
	http.HandleFunc("/v1/orders/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		orderID := r.URL.Path[len("/v1/orders/"):]
		resp := map[string]any{
			"order_id":   orderID,
			"status":     "shipped",
			"total":      199.99,
			"items":      []string{"商品A", "商品B"},
			"created_at": "2024-03-15T10:30:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Logistics endpoint
	http.HandleFunc("/v1/logistics/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		trackingNumber := r.URL.Path[len("/v1/logistics/"):]
		resp := map[string]any{
			"tracking_number": trackingNumber,
			"status":          "运输中",
			"carrier":         "顺丰快递",
			"events": []map[string]string{
				{
					"time":     "2024-03-15 14:30:00",
					"status":   "已发出",
					"location": "上海分拨中心",
				},
				{
					"time":     "2024-03-15 10:00:00",
					"status":   "已揽收",
					"location": "上海浦东站点",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	log.Println("HTTP Mock Server starting on :28081")
	log.Println("Available endpoints:")
	log.Println("  - GET  /v1/orders/{order_id}")
	log.Println("  - GET  /v1/logistics/{tracking_number}")
	log.Fatal(http.ListenAndServe(":28081", nil))
}

// Example curl commands:
// curl http://localhost:28081/v1/orders/20240315001
// curl http://localhost:28081/v1/logistics/SF123456789

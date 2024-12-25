package main

import (
	"encoding/json"
	"net/http"
)

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	var client Client
	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Process the client data...
	w.WriteHeader(http.StatusOK)
}

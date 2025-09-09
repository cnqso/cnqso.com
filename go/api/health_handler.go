package api

import (
	"encoding/json"
	"net/http"
	"server/types"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(types.JSONResponse{
		Success: true,
		Message: "OK",
	})
}

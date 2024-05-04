package healthcheck

import (
	"encoding/json"
	"net/http"
	"os"
)

type Health struct {
	Healthy  bool   `json:"healthy"`
	Hostname string `json:"hostname"`
}

func HealthcheckHandler(w http.ResponseWriter, r *http.Request) {
	hostname, err := os.Hostname()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(
		&Health{
			Healthy:  true,
			Hostname: hostname,
		},
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

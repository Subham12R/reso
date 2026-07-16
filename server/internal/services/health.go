package services

type HealthResponse struct {
	Status string `json:"status"`
}

func Health() HealthResponse {
	return HealthResponse{Status: "ok"}
}

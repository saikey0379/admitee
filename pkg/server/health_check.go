package server

import (
	"encoding/json"
	"github.com/golang/glog"
	"net/http"
	"strconv"
)

type WResponse struct {
	Status  string `json:"status"` //response
	Message string `json:"message"`
}

func (s *apiServer) HealthCheck(w http.ResponseWriter, r *http.Request) {
	var status = http.StatusOK
	var data []byte
	if !s.clientRedis.Health {
		data, _ = json.Marshal(WResponse{Status: "down"})
		status = http.StatusServiceUnavailable
		glog.Errorf("FAILURE: Redis unhealth[%v]", s.clientRedis.Health)
	} else {
		data, _ = json.Marshal(WResponse{Status: "up"})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	w.Write(data)
}

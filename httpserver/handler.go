package httpserver

import (
	"encoding/hex"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/go-chi/chi/v5"
	"github.com/google/go-tdx-guest/client"
	"github.com/ruteri/dummy-tdx-dcap/metrics"
)

func TdxAttest(appdata [64]byte) ([]byte, error) {
	qp := &client.LinuxConfigFsQuoteProvider{}
	return qp.GetRawQuote(appdata)
}

func DummyAttest(appdata [64]byte) ([]byte, error) {
	return appdata[:], nil
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	m := s.metricsSrv.Float64Histogram(
		"request_duration_api",
		"API request handling duration",
		metrics.UomMicroseconds,
		metrics.BucketsRequestDuration...,
	)
	defer func(start time.Time) {
		m.Record(r.Context(), float64(time.Since(start).Microseconds()))
	}(time.Now())

	urlAppdata := chi.URLParam(r, "appdata")

	// Decode hex string to bytes
	decodedAppData, err := hex.DecodeString(urlAppdata)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var appdata [64]byte
	copy(appdata[:], decodedAppData)

	// Call attestFn with the decoded bytes
	quote, err := s.attestFn(appdata)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(quote)
	if err != nil {
		log.Error("could not send back the quote", "err", err)
	}
}

func (s *Server) handleLivenessCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReadinessCheck(w http.ResponseWriter, r *http.Request) {
	if !s.isReady.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDrain(w http.ResponseWriter, r *http.Request) {
	if wasReady := s.isReady.Swap(false); !wasReady {
		return
	}
	// l := logutils.ZapFromRequest(r)
	s.log.Info("Server marked as not ready")
	time.Sleep(s.cfg.DrainDuration) // Give LB enough time to detect us not ready
}

func (s *Server) handleUndrain(w http.ResponseWriter, r *http.Request) {
	if wasReady := s.isReady.Swap(true); wasReady {
		return
	}
	// l := logutils.ZapFromRequest(r)
	s.log.Info("Server marked as ready")
}

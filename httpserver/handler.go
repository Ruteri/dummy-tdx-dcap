package httpserver

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	tdx_abi "github.com/google/go-tdx-guest/abi"
	"github.com/google/go-tdx-guest/client"
	tdx_pb "github.com/google/go-tdx-guest/proto/tdx"
	"github.com/google/go-tdx-guest/verify"
	"github.com/ruteri/dummy-tdx-dcap/metrics"
)

func TdxAttest(appdata [64]byte) ([]byte, error) {
	qp := &client.LinuxConfigFsQuoteProvider{}
	return qp.GetRawQuote(appdata)
}

func DummyAttest(appdata [64]byte) ([]byte, error) {
	return appdata[:], nil
}

func (s *Server) handleAttest(w http.ResponseWriter, r *http.Request) {
	m := s.metricsSrv.Float64Histogram(
		"request_duration_api",
		"attest handling duration",
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
		s.log.Error("could not send back the quote", "err", err)
	}
}

const MaxQuoteSize = 12800

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	m := s.metricsSrv.Float64Histogram(
		"request_duration_api",
		"verify handling duration",
		metrics.UomMicroseconds,
		metrics.BucketsRequestDuration...,
	)
	defer func(start time.Time) {
		m.Record(r.Context(), float64(time.Since(start).Microseconds()))
	}(time.Now())

	if r.Body == nil {
		http.Error(w, "no quote", http.StatusBadRequest)
		return
	}

	rawQuoteData, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxQuoteSize))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	protoQuote, err := tdx_abi.QuoteToProto(rawQuoteData[:])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	v4Quote, err := func() (*tdx_pb.QuoteV4, error) {
		switch q := protoQuote.(type) {
		case *tdx_pb.QuoteV4:
			return q, nil
		default:
			return nil, fmt.Errorf("unsupported quote type: %T", q)
		}
	}()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.log.Info("quote", "quote", v4Quote)

	options := verify.DefaultOptions()
	// TODO: fetch collateral before verifying to distinguish the error better
	err = verify.TdxQuote(protoQuote, options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusTeapot)
		return
	}

	err = json.NewEncoder(w).Encode(v4Quote)
	if err != nil {
		s.log.Error("could not respond to /verify", "err", err)
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

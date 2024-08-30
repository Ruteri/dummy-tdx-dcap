package httpserver

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ruteri/dummy-tdx-dcap/common"
	"github.com/stretchr/testify/require"
)

func getTestLogger() *slog.Logger {
	return common.SetupLogger(&common.LoggingOpts{
		Debug:   true,
		JSON:    false,
		Service: "test",
		Version: "test",
	})
}

var (
	//go:embed sample_quote.hex
	quoteData string
	//go:embed sample_result.json
	expectedVerifyResult string
)

func Test_verify_sample(t *testing.T) {
	// TODO: might start failing at some point once TCB is outdated. Would be great to freeze the collateral.
	const (
		latency    = 200 * time.Millisecond
		listenAddr = ":8080"
	)

	//nolint: exhaustruct
	s, err := New(&HTTPServerConfig{
		DrainDuration: latency,
		ListenAddr:    listenAddr,
		Log:           getTestLogger(),
	})
	require.NoError(t, err)

	rawQuote, err := hex.DecodeString(quoteData)
	require.NoError(t, err)

	{ // Check health
		req := httptest.NewRequest(http.MethodPost, "http://localhost"+listenAddr+"/verify", bytes.NewReader(rawQuote))
		w := httptest.NewRecorder()
		s.handleVerify(w, req)
		resp := w.Result()
		defer resp.Body.Close()
		responseBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Verification should pass")
		require.Equal(t, []byte(expectedVerifyResult), responseBody)
	}
}

func Test_Handlers_Healthcheck_Drain_Undrain(t *testing.T) {
	const (
		latency    = 200 * time.Millisecond
		listenAddr = ":8080"
	)

	//nolint: exhaustruct
	s, err := New(&HTTPServerConfig{
		DrainDuration: latency,
		ListenAddr:    listenAddr,
		Log:           getTestLogger(),
	})
	require.NoError(t, err)

	{ // Check health
		req := httptest.NewRequest(http.MethodGet, "http://localhost"+listenAddr+"/readyz", nil) //nolint:goconst,nolintlint
		w := httptest.NewRecorder()
		s.handleReadinessCheck(w, req)
		resp := w.Result()
		defer resp.Body.Close()
		_, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Healthcheck must return `Ok` before draining")
	}

	{ // Drain
		req := httptest.NewRequest(http.MethodGet, "http://localhost"+listenAddr+"/drain", nil)
		w := httptest.NewRecorder()
		start := time.Now()
		s.handleDrain(w, req)
		duration := time.Since(start)
		resp := w.Result()
		defer resp.Body.Close()
		_, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Must return `Ok` for calls to `/drain`")
		require.GreaterOrEqual(t, duration, latency, "Must wait long enough during draining")
	}

	{ // Check health
		req := httptest.NewRequest(http.MethodGet, "http://localhost"+listenAddr+"/readyz", nil)
		w := httptest.NewRecorder()
		s.handleReadinessCheck(w, req)
		resp := w.Result()
		defer resp.Body.Close()
		_, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "Healthcheck must return `Service Unavailable` after draining")
	}

	{ // Undrain
		req := httptest.NewRequest(http.MethodGet, "http://localhost"+listenAddr+"/undrain", nil)
		w := httptest.NewRecorder()
		s.handleUndrain(w, req)
		resp := w.Result()
		defer resp.Body.Close()
		_, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Must return `Ok` for calls to `/undrain`")
		time.Sleep(latency)
	}

	{ // Check health
		req := httptest.NewRequest(http.MethodGet, "http://localhost"+listenAddr+"/readyz", nil)
		w := httptest.NewRecorder()
		s.handleReadinessCheck(w, req)
		resp := w.Result()
		defer resp.Body.Close()
		_, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Healthcheck must return `Ok` after undraining")
	}
}

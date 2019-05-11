package server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/f110/command-server/pkg/config"
)

func TestCommandServer_ServeHTTP(t *testing.T) {
	commands := []config.Command{
		{
			Name:    "test",
			Command: []string{"echo", "ok"},
		},
		{
			Name:    "timeout",
			Command: []string{"sleep", "2"},
			Timeout: 1,
		},
		{
			Name:    "unknown",
			Command: []string{"command-not-found"},
		},
		{
			Name:      "sequence",
			Command:   []string{"sleep", "1"},
			Exclusion: true,
		},
	}
	s := NewCommandServer(commands)

	t.Run("execute command", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(NewCommandRequest{Name: "test"}); err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/new", &buf)
		s.ServeHTTP(w, req)
		res := w.Result()
		if res.StatusCode != http.StatusOK {
			t.Errorf("expect status ok but got: %s", res.Status)
		}
	})

	t.Run("with args", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(NewCommandRequest{Name: "test", Args: []string{"good"}}); err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/new", &buf)
		s.ServeHTTP(w, req)
		res := w.Result()

		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "ok good\n" {
			t.Error("unexpected output")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(NewCommandRequest{Name: "timeout"}); err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/new", &buf))
		res := w.Result()
		if res.StatusCode != http.StatusOK {
			t.Errorf("expect status ok but got: %s", res.Status)
		}
	})

	t.Run("command not found", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(NewCommandRequest{Name: "unknown"}); err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/new", &buf))
		res := w.Result()
		if res.StatusCode != http.StatusInternalServerError {
			t.Errorf("expect status ISE but got: %s", res.Status)
		}
	})

	t.Run("sequence", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(NewCommandRequest{Name: "sequence"}); err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/new", bytes.NewReader(buf.Bytes())))
	})

	t.Run("status", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(NewCommandRequest{Name: "test"}); err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/new", &buf)
		s.ServeHTTP(w, req)
		res := w.Result()

		id := res.Header.Get("X-Status-Id")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/status/"+id, nil))
		res = w.Result()

		var commandStatus status
		if err := json.NewDecoder(res.Body).Decode(&commandStatus); err != nil {
			t.Fatal(err)
		}
		if commandStatus.Name != "test" {
			t.Errorf("expect test but got: %s", commandStatus.Name)
		}
	})

	t.Run("not allowed method", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		for _, target := range []string{"/new", "/status/1"} {
			req := httptest.NewRequest(http.MethodGet, target, nil)
			s.ServeHTTP(w, req)
			res := w.Result()
			if res.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("expect status method not allowed but got %s", res.Status)
			}
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(NewCommandRequest{Name: "not-exist"}); err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/new", &buf)
		s.ServeHTTP(w, req)
		res := w.Result()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("expect status not found but got: %s", res.Status)
		}
	})

	t.Run("invalid request to get status", func(t *testing.T) {
		t.Parallel()

		for _, target := range []string{"/status/", "/status/hoge", "/status/10000000"} {
			w := httptest.NewRecorder()
			s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, target, nil))
			res := w.Result()

			if res.StatusCode != http.StatusBadRequest && res.StatusCode != http.StatusNotFound {
				t.Errorf("expect status bad request or not found but got %s", res.Status)
			}
		}
	})
}

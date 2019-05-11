package server

import "net/http"

type Writer struct {
	w http.ResponseWriter
	f http.Flusher
}

func NewWriter(w http.ResponseWriter) *Writer {
	writer := &Writer{w: w}
	if f, ok := w.(http.Flusher); ok {
		writer.f = f
	}
	return writer
}

func (w *Writer) Write(b []byte) (int, error) {
	defer func() {
		if w.f != nil {
			w.f.Flush()
		}
	}()

	return w.w.Write(b)
}

package httpserver

import (
	"net/http"
	"net/http/pprof"

	"github.com/stephane-martin/skewer/sys/binder"
)

type HTTPServer struct {
	srv *http.Server
}

func (s *HTTPServer) ListenAndServe(b binder.Client) error {
	ln, err := b.Listen("tcp", "127.0.0.1:6060")
	if err != nil {
		return err
	}
	return s.srv.Serve(ln)
}

func ProfileServer(b binder.Client) {
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/pprof/heap", pprof.Handler("heap"))
		mux.Handle("/pprof/profile", http.HandlerFunc(pprof.Profile))
		server := &HTTPServer{
			srv: &http.Server{
				Handler: mux,
			},
		}
		server.ListenAndServe(b)
	}()
}

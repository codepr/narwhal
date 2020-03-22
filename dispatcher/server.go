package dispatcher

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"
)

type Server struct {
	server              *http.Server
	runnerPool          *TestRunnerPool
	healthcheck_ch      chan bool
	healthcheck_timeout time.Duration
}

func newRouter(p *TestRunnerPool) *http.ServeMux {
	router := http.NewServeMux()
	router.Handle("/testrunner", handleTestRunner(p))
	router.Handle("/commit", handleCommit(p))
	return router
}

func NewServer(addr string, l *log.Logger,
	p *TestRunnerPool, ts time.Duration) *Server {
	return &Server{
		server: &http.Server{
			Addr:           addr,
			Handler:        logReq(l)(newRouter(p)),
			ErrorLog:       l,
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
		runnerPool:          p,
		healthcheck_timeout: ts,
		healthcheck_ch:      make(chan bool),
	}
}

func (s *Server) Run() error {
	done := make(chan bool)
	quit := make(chan os.Signal, 1)

	go func() {
		<-quit
		s.server.ErrorLog.Println("Shutdown")
		// Stop the healthcheck goroutine
		s.healthcheck_ch <- true
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.server.SetKeepAlivesEnabled(false)
		if err := s.server.Shutdown(ctx); err != nil {
			s.server.ErrorLog.Fatal("Could not shutdown the server")
		}
		close(done)
	}()

	// Start healthcheck goroutine
	go s.runnersHealthcheck()

	s.server.ErrorLog.Println("Listening on", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.server.ErrorLog.Println("Unable to bind on %s", s.server.Addr)
	}

	<-done
	return nil
}

func (s *Server) runnersHealthcheck() {
	ticker := time.NewTicker(s.healthcheck_timeout * time.Second)
	for {
		select {
		case <-s.healthcheck_ch:
			ticker.Stop()
			return
		case <-ticker.C:
			for _, t := range s.runnerPool.runners {
				res, err := http.Get(t.URL + "/health")
				if err != nil || res.StatusCode != 200 {
					t.Alive = false
				}
			}
		}
	}
}

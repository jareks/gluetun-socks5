package socks5

import (
	"context"
	"sync"
	"time"
	"github.com/txthinking/socks5"
)

type Server struct {
	address           string
	username 					string
	password 					string
	// handler           http.Handler
	logger            infoErrorer
	internalWG        *sync.WaitGroup
	// readHeaderTimeout time.Duration
	readTimeout       time.Duration
}

func New(ctx context.Context, address string, logger Logger,
	username, password string, readTimeout time.Duration,
) *Server {
	wg := &sync.WaitGroup{}
	return &Server{
		address:           address,
		username: 				 username,
		password: 				 password,
		logger:            logger,
		internalWG:        wg,
	}
}

func (s *Server) Run(ctx context.Context, errorCh chan<- error) {
	server, err := socks5.NewClassicServer(s.address, "0.0.0.0", s.username, s.password, 0, 60) // TODO: what is 0.0.0.0 here? add tcp and udp read timeouts
	if err != nil {
		s.logger.Error("failed creating socks5 server: " + err.Error())
		errorCh <- err
		return
	}

	go func() {
		<-ctx.Done()
		const shutdownGraceDuration = 100 * time.Millisecond
		//shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGraceDuration)
		// defer cancel()
		if err := server.Shutdown(); err != nil {
			s.logger.Error("failed shutting down: " + err.Error())
		}
	}()
	s.logger.Info("listening on " + s.address)
	err = server.ListenAndServe(nil) // nil means default handler
	server.RunnerGroup.Wait()

	if err != nil && ctx.Err() == nil {
		errorCh <- err
	} else {
		errorCh <- nil
	}
}

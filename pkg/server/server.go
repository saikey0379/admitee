package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"time"

	"admitee/pkg/model"
	"admitee/pkg/server/config"

	"github.com/golang/glog"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type apiServer struct {
	config            *config.Config
	clientKubeDynamic dynamic.Interface
	clientKubeSet     *kubernetes.Clientset
	clientRedis       *model.AdmiteeRedisClient
	Server            *http.Server
	stopCh            chan struct{}
}

func NewServer(cfg *config.Config, clientKubeDynamic dynamic.Interface, clientKubeSet *kubernetes.Clientset, clientRedis *model.AdmiteeRedisClient) (*apiServer, error) {
	server := &apiServer{
		config:            cfg,
		clientKubeDynamic: clientKubeDynamic,
		clientKubeSet:     clientKubeSet,
		clientRedis:       clientRedis,
	}

	return server, nil
}

func (s *apiServer) Run(ctx context.Context) {
	s.startGracefulShutDown(ctx)

	pair, err := tls.LoadX509KeyPair(s.config.TlsCert, s.config.TlsKey)
	if err != nil {
		glog.Errorf("FAILURE: Failed to load key pair: %v", err)
	}

	s.DeamonSmooth()
	go s.clientRedis.HealthCheckRdb()

	go func() {
		s.Server = &http.Server{
			Addr:         net.JoinHostPort(s.config.BindAddress, strconv.Itoa(s.config.BindPort)),
			TLSConfig:    &tls.Config{Certificates: []tls.Certificate{pair}},
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		// define http server and server handler
		mux := http.NewServeMux()
		// mux.HandleFunc("/mutate", whsvr.serve)
		mux.HandleFunc("/admission/smooth", s.Admission)
		mux.HandleFunc("/healthz", s.HealthCheck)
		s.Server.Handler = mux

		glog.Infof("Start to listening on http address: %s", s.Server.Addr)

		if err := s.Server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			glog.Fatal(err)
		}
		glog.Infof("Stop to listening on http address: %s", s.Server.Addr)

	}()

	<-s.stopCh
	glog.Infof("Server on %s stopped", s.Server.Addr)
}

func (s *apiServer) startGracefulShutDown(ctx context.Context) {
	go func() {
		<-ctx.Done()
		s.Close()
		s.stopCh <- struct{}{}
	}()
}

// Close graceful shutdown.
func (s *apiServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.Server.Shutdown(ctx); err != nil {
		glog.Warningf("Shutdown insecure server failed: %s", err.Error())
	}
}

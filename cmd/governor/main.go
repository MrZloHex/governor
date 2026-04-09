package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	log "log/slog"

	cli "github.com/spf13/pflag"

	"governor/internal/governor"
	"governor/pkg/proto"
)

var logLevelMap = map[string]log.Level{
	"debug": log.LevelDebug,
	"info":  log.LevelInfo,
	"warn":  log.LevelWarn,
	"error": log.LevelError,
}

func loadDotEnv() {
	err := godotenv.Load()
	if err == nil {
		return
	}
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	var pe *os.PathError
	if errors.As(err, &pe) && errors.Is(pe.Err, os.ErrNotExist) {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "governor: warning: .env: %v\n", err)
}

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func buildClientTLSConfig(certFile, keyFile, serverCAFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load client certificate: %w", err)
	}

	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if serverCAFile != "" {
		pem, err := os.ReadFile(serverCAFile)
		if err != nil {
			return nil, fmt.Errorf("read server CA bundle: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no certificates parsed from server CA file %q", serverCAFile)
		}
		cfg.RootCAs = pool
	}

	return cfg, nil
}

func maybeWSS(url string, useTLS bool) string {
	if !useTLS {
		return url
	}
	if strings.HasPrefix(url, "ws://") {
		return "wss://" + strings.TrimPrefix(url, "ws://")
	}
	return url
}

func main() {
	loadDotEnv()

	defaultURL := envString("GOVERNOR_WS_URL", "ws://localhost:8092")
	defaultCert := os.Getenv("GOVERNOR_TLS_CERT")
	defaultKey := os.Getenv("GOVERNOR_TLS_KEY")
	defaultCA := os.Getenv("GOVERNOR_TLS_CA")

	url := cli.StringP("url", "u", defaultURL, "WebSocket hub URL (env GOVERNOR_WS_URL)")
	logLevel := cli.StringP("log", "l", "info", "Log level")
	schedulePath := cli.StringP("schedule", "s", "weekly_schedule.csv", "Path to weekly schedule CSV")
	eventsPath := cli.StringP("events", "e", "events.json", "Path to events persistence file")
	tlsCert := cli.String("tls-cert", defaultCert, "Path to client TLS certificate (PEM) for mTLS (env GOVERNOR_TLS_CERT)")
	tlsKey := cli.String("tls-key", defaultKey, "Path to client TLS private key (PEM) (env GOVERNOR_TLS_KEY)")
	tlsCA := cli.String("tls-ca", defaultCA, "Path to PEM bundle of CAs for verifying the hub server cert (env GOVERNOR_TLS_CA)")
	cli.Parse()

	log.SetDefault(log.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level: logLevelMap[*logLevel],
	})))

	hubURL := *url
	var opts []proto.Option
	opts = append(opts, proto.WithReconnect(5*time.Second))

	if *tlsCert != "" || *tlsKey != "" || *tlsCA != "" {
		if *tlsCert == "" || *tlsKey == "" {
			log.Error("TLS incomplete: --tls-cert and --tls-key are required for mTLS")
			os.Exit(1)
		}
		tlsCfg, err := buildClientTLSConfig(*tlsCert, *tlsKey, *tlsCA)
		if err != nil {
			log.Error("TLS configuration failed", "err", err)
			os.Exit(1)
		}
		opts = append(opts, proto.WithTLS(tlsCfg))
		hubURL = maybeWSS(hubURL, true)
		log.Info("connecting with mTLS", "url", hubURL, "cert", *tlsCert)
	}

	client := proto.New("GOVERNOR", hubURL, opts...)

	gov, err := governor.New(client, *schedulePath, *eventsPath)
	if err != nil {
		log.Error("Failed to init governor", "err", err)
		os.Exit(1)
	}

	client.Handle("*", func(req *proto.Request) {
		if req.Msg.To != client.NodeID() {
			return
		}
		gov.Cmd(req)
	})

	log.Info("BOOTING UP", "url", hubURL)

	if err := client.Connect(context.Background()); err != nil {
		log.Error("Failed to connect", "err", err)
		os.Exit(1)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info("SHUTTING DOWN")
	client.Close()
	gov.Shutdown()
}

package grpcserver

import (
	"os"
	"time"
)

const (
	defaultAddress   = ":9999"
	defaultMaxRPC    = 600
	defaultTimeout   = time.Minute
	defaultNamespace = "ndd-system"
)

type Config struct {
	// gRPC server address
	Address string
	// insecure server
	Insecure bool
	// secret name holding the server certificate
	CertificateSecret string
	// secret name holding ca certificates
	CaCertificateSecret string
	// MaxRPC
	MaxRPC int64
	// services config
	GNMI   bool
	Health bool
	// namespace where the certificates secret is created.
	// defaults to $POD_NAMESPACE
	Namespace string
	// request timeout
	Timeout time.Duration
}

func (c *Config) setDefaults() {
	if c.Address == "" {
		c.Address = ":9999"
	}
	if c.MaxRPC <= 0 {
		c.MaxRPC = defaultMaxRPC
	}
	if c.Namespace == "" {
		ns, ok := os.LookupEnv("POD_NAMESPACE")
		if !ok || ns == "" {
			ns = defaultNamespace
		}
		c.Namespace = ns
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
}
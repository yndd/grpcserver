package grpcserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (s *GrpcServer) serverOpts(ctx context.Context) ([]grpc.ServerOption, error) {
	if s.config.Insecure {
		return []grpc.ServerOption{
			grpc.Creds(insecure.NewCredentials()),
		}, nil
	}

	tlsConfig, err := s.createTLSConfig(ctx)
	if err != nil {
		return nil, err
	}
	return []grpc.ServerOption{
		grpc.Creds(credentials.NewTLS(tlsConfig)),
	}, nil

}

func (s *GrpcServer) createTLSConfig(ctx context.Context) (*tls.Config, error) {
	lookupname, ok := os.LookupEnv("GRPC_CERT_SECRET_NAME")
	if !ok {
		s.logger.Debug("grpc server createTLSConfig", "lookupName nok", lookupname)
	}
	s.logger.Debug("grpc server createTLSConfig", "lookup name", lookupname, "env name", os.Getenv("GRPC_CERT_SECRET_NAME"))
	s.logger.Debug("grpc server createTLSConfig", "namespace", s.config.Namespace, "name", s.config.CaCertificateSecret)
	caCert := &corev1.Secret{}
	err := s.client.Get(ctx, types.NamespacedName{
		Namespace: s.config.Namespace,
		Name:      s.config.CertificateSecret,
	}, caCert)
	if err != nil {
		return nil, err
	}
	ca := caCert.Data["ca.crt"]

	tlsConfig := &tls.Config{
		GetCertificate: s.readCerts,
	}
	if len(ca) != 0 {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(ca)
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

func (s *GrpcServer) readCerts(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	now := time.Now()
	s.cm.Lock()
	defer s.cm.Unlock()
	if now.After(s.lastRead.Add(time.Minute)) {
		certs := &corev1.Secret{}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := s.client.Get(ctx, types.NamespacedName{
			Namespace: s.config.Namespace,
			Name:      s.config.CertificateSecret,
		}, certs)
		if err != nil {
			return nil, err
		}
		key := certs.Data["tls.key"]
		cert := certs.Data["tls.crt"]
		serverCert, err := tls.X509KeyPair(cert, key)
		if err != nil {
			return nil, err
		}
		s.cert = &serverCert
		s.lastRead = time.Now()
	}

	return s.cert, nil
}

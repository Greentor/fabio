package cert

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/eBay/fabio/config"
)

// Source provides the interface for dynamic certificate sources.
//
// Certificates() loads certificates for TLS connections.
// The first certificate is used as the default certificate
// if the client does not support SNI or no matching certificate
// could be found. TLS certificates can be updated at runtime.
//
// LoadClientCAs() provides certificates for client certificate
// authentication.
type Source interface {
	Certificates() chan []tls.Certificate
	LoadClientCAs() (*x509.CertPool, error)
}

// NewSource generates a cert source from the config options.
func NewSource(cfg config.CertSource) (Source, error) {
	switch cfg.Type {
	case "file":
		return FileSource{
			CertFile:       cfg.CertPath,
			KeyFile:        cfg.KeyPath,
			ClientAuthFile: cfg.ClientCAPath,
			CAUpgradeCN:    cfg.CAUpgradeCN,
		}, nil

	case "path":
		return PathSource{
			CertPath:     cfg.CertPath,
			ClientCAPath: cfg.ClientCAPath,
			CAUpgradeCN:  cfg.CAUpgradeCN,
			Refresh:      cfg.Refresh,
		}, nil

	case "http":
		return HTTPSource{
			CertURL:     cfg.CertPath,
			ClientCAURL: cfg.ClientCAPath,
			CAUpgradeCN: cfg.CAUpgradeCN,
			Refresh:     cfg.Refresh,
		}, nil

	case "consul":
		return ConsulSource{
			CertURL:     cfg.CertPath,
			ClientCAURL: cfg.ClientCAPath,
			CAUpgradeCN: cfg.CAUpgradeCN,
		}, nil

	case "vault":
		return VaultSource{
			// TODO(fs): configure Addr but not token
			CertPath:     cfg.CertPath,
			ClientCAPath: cfg.ClientCAPath,
			CAUpgradeCN:  cfg.CAUpgradeCN,
			Refresh:      cfg.Refresh,
		}, nil

	default:
		return nil, fmt.Errorf("invalid certificate source %q", cfg.Type)
	}
}

// TLSConfig creates a tls.Config which sets the
// GetCertificate field to a certificate store
// which uses the given source to update the
// the certificates on demand.
//
// It also sets the ClientCAs field if
// src.LoadClientCAs returns a non-nil value
// and sets ClientAuth to RequireAndVerifyClientCert.
func TLSConfig(src Source) (*tls.Config, error) {
	clientCAs, err := src.LoadClientCAs()
	if err != nil {
		return nil, err
	}

	store := NewStore()
	x := &tls.Config{GetCertificate: store.GetCertificate}

	if clientCAs != nil {
		x.ClientCAs = clientCAs
		x.ClientAuth = tls.RequireAndVerifyClientCert
	}

	go func() {
		for certs := range src.Certificates() {
			store.SetCertificates(certs)
		}
	}()

	return x, nil
}

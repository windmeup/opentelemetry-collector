// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configtls // import "go.opentelemetry.io/collector/config/configtls"

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// We should avoid that users unknowingly use a vulnerable TLS version.
// The defaults should be a safe configuration
const defaultMinTLSVersion = tls.VersionTLS12

// Uses the default MaxVersion from "crypto/tls" which is the maximum supported version
const defaultMaxTLSVersion = 0

// TLSSetting exposes the common client and server TLS configurations.
// Note: Since there isn't anything specific to a server connection. Components
// with server connections should use TLSSetting.
type TLSSetting struct {
	// Path to the CA cert. For a client this verifies the server certificate.
	// For a server this verifies client certificates. If empty uses system root CA.
	// (optional)
	CAFile string `mapstructure:"ca_file"`

	// Path to the TLS cert to use for TLS required connections. (optional)
	CertFile string `mapstructure:"cert_file"`

	// Path to the TLS key to use for TLS required connections. (optional)
	KeyFile string `mapstructure:"key_file"`

	// MinVersion sets the minimum TLS version that is acceptable.
	// If not set, TLS 1.2 will be used. (optional)
	MinVersion string `mapstructure:"min_version"`

	// MaxVersion sets the maximum TLS version that is acceptable.
	// If not set, refer to crypto/tls for defaults. (optional)
	MaxVersion string `mapstructure:"max_version"`

	// ReloadInterval specifies the duration after which the certificate will be reloaded
	// If not set, it will never be reloaded (optional)
	ReloadInterval time.Duration `mapstructure:"reload_interval"`
}

// TLSClientSetting contains TLS configurations that are specific to client
// connections in addition to the common configurations. This should be used by
// components configuring TLS client connections.
type TLSClientSetting struct {
	// squash ensures fields are correctly decoded in embedded struct.
	TLSSetting `mapstructure:",squash"`

	// These are config options specific to client connections.

	// In gRPC when set to true, this is used to disable the client transport security.
	// See https://godoc.org/google.golang.org/grpc#WithInsecure.
	// In HTTP, this disables verifying the server's certificate chain and host name
	// (InsecureSkipVerify in the tls Config). Please refer to
	// https://godoc.org/crypto/tls#Config for more information.
	// (optional, default false)
	Insecure bool `mapstructure:"insecure"`
	// InsecureSkipVerify will enable TLS but not verify the certificate.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
	// ServerName requested by client for virtual hosting.
	// This sets the ServerName in the TLSConfig. Please refer to
	// https://godoc.org/crypto/tls#Config for more information. (optional)
	ServerName string `mapstructure:"server_name_override"`
}

// TLSServerSetting contains TLS configurations that are specific to server
// connections in addition to the common configurations. This should be used by
// components configuring TLS server connections.
type TLSServerSetting struct {
	// squash ensures fields are correctly decoded in embedded struct.
	TLSSetting `mapstructure:",squash"`

	// These are config options specific to server connections.

	// Path to the TLS cert to use by the server to verify a client certificate. (optional)
	// This sets the ClientCAs and ClientAuth to RequireAndVerifyClientCert in the TLSConfig. Please refer to
	// https://godoc.org/crypto/tls#Config for more information. (optional)
	ClientCAFile string `mapstructure:"client_ca_file"`

	// Reload the ClientCAs file when it is modified
	// (optional, default false)
	ReloadClientCAFile bool `mapstructure:"client_ca_file_reload"`
}

// certReloader is a wrapper object for certificate reloading
// Its GetCertificate method will either return the current certificate or reload from disk
// if the last reload happened more than ReloadInterval ago
type certReloader struct {
	// Path to the TLS cert
	CertFile string
	// Path to the TLS key
	KeyFile string
	// ReloadInterval specifies the duration after which the certificate will be reloaded
	// If not set, it will never be reloaded (optional)
	ReloadInterval time.Duration
	nextReload     time.Time
	cert           *tls.Certificate
	lock           sync.RWMutex
}

func newCertReloader(certFile, keyFile string, reloadInterval time.Duration) (*certReloader, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &certReloader{
		CertFile:       certFile,
		KeyFile:        keyFile,
		ReloadInterval: reloadInterval,
		nextReload:     time.Now().Add(reloadInterval),
		cert:           &cert,
	}, nil
}

func (r *certReloader) GetCertificate() (*tls.Certificate, error) {
	now := time.Now()
	// Read locking here before we do the time comparison
	// If a reload is in progress this will block and we will skip reloading in the current
	// call once we can continue
	r.lock.RLock()
	if r.ReloadInterval != 0 && r.nextReload.Before(now) {
		// Need to release the read lock, otherwise we deadlock
		r.lock.RUnlock()
		r.lock.Lock()
		defer r.lock.Unlock()
		cert, err := tls.LoadX509KeyPair(r.CertFile, r.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS cert and key: %w", err)
		}
		r.cert = &cert
		r.nextReload = now.Add(r.ReloadInterval)
		return r.cert, nil
	}
	defer r.lock.RUnlock()
	return r.cert, nil
}

// LoadTLSConfig loads TLS certificates and returns a tls.Config.
// This will set the RootCAs and Certificates of a tls.Config.
func (c TLSSetting) loadTLSConfig() (*tls.Config, error) {
	// There is no need to load the System Certs for RootCAs because
	// if the value is nil, it will default to checking against th System Certs.
	var err error
	var certPool *x509.CertPool
	if len(c.CAFile) != 0 {
		// Set up user specified truststore.
		certPool, err = c.loadCert(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA CertPool: %w", err)
		}
	}

	if (c.CertFile == "" && c.KeyFile != "") || (c.CertFile != "" && c.KeyFile == "") {
		return nil, errors.New("for auth via TLS, either both certificate and key must be supplied, or neither")
	}

	var getCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	var getClientCertificate func(*tls.CertificateRequestInfo) (*tls.Certificate, error)
	if c.CertFile != "" && c.KeyFile != "" {
		var certReloader *certReloader
		certReloader, err = newCertReloader(c.CertFile, c.KeyFile, c.ReloadInterval)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS cert and key: %w", err)
		}
		getCertificate = func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) { return certReloader.GetCertificate() }
		getClientCertificate = func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) { return certReloader.GetCertificate() }
	}

	minTLS, err := convertVersion(c.MinVersion, defaultMinTLSVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid TLS min_version: %w", err)
	}
	maxTLS, err := convertVersion(c.MaxVersion, defaultMaxTLSVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid TLS max_version: %w", err)
	}

	return &tls.Config{
		RootCAs:              certPool,
		GetCertificate:       getCertificate,
		GetClientCertificate: getClientCertificate,
		MinVersion:           minTLS,
		MaxVersion:           maxTLS,
	}, nil
}

func (c TLSSetting) loadCert(caPath string) (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(filepath.Clean(caPath))
	if err != nil {
		return nil, fmt.Errorf("failed to load CA %s: %w", caPath, err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA %s", caPath)
	}
	return certPool, nil
}

// LoadTLSConfig loads the TLS configuration.
func (c TLSClientSetting) LoadTLSConfig() (*tls.Config, error) {
	if c.Insecure && c.CAFile == "" {
		return nil, nil
	}

	tlsCfg, err := c.TLSSetting.loadTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS config: %w", err)
	}
	tlsCfg.ServerName = c.ServerName
	tlsCfg.InsecureSkipVerify = c.InsecureSkipVerify
	return tlsCfg, nil
}

// LoadTLSConfig loads the TLS configuration.
func (c TLSServerSetting) LoadTLSConfig() (*tls.Config, error) {
	tlsCfg, err := c.loadTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS config: %w", err)
	}
	if c.ClientCAFile != "" {
		reloader, err := newClientCAsReloader(c.ClientCAFile, &c)
		if err != nil {
			return nil, err
		}
		if c.ReloadClientCAFile {
			err = reloader.startWatching()
			if err != nil {
				return nil, err
			}
			tlsCfg.GetConfigForClient = func(t *tls.ClientHelloInfo) (*tls.Config, error) { return reloader.getClientConfig(tlsCfg) }
		}
		tlsCfg.ClientCAs = reloader.certPool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return tlsCfg, nil
}

func (c TLSServerSetting) loadClientCAFile() (*x509.CertPool, error) {
	return c.loadCert(c.ClientCAFile)
}

func convertVersion(v string, defaultVersion uint16) (uint16, error) {
	// Use a default that is explicitly defined
	if v == "" {
		return defaultVersion, nil
	}
	val, ok := tlsVersions[v]
	if !ok {
		return 0, fmt.Errorf("unsupported TLS version: %q", v)
	}
	return val, nil
}

var tlsVersions = map[string]uint16{
	"1.0": tls.VersionTLS10,
	"1.1": tls.VersionTLS11,
	"1.2": tls.VersionTLS12,
	"1.3": tls.VersionTLS13,
}

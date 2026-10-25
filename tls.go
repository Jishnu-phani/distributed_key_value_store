package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"os"
	"time"
)

// loadIdentityAndCA reads this node's certificate/key pair and the cluster CA
// from the paths given by TLS_CERT_FILE, TLS_KEY_FILE and TLS_CA_FILE.
// Both the server and the client side of mTLS need exactly these two things:
// "who am I?" (cert+key) and "which issuer do I trust?" (CA pool).
func loadIdentityAndCA() (tls.Certificate, *x509.CertPool, error) {
	cert, err := tls.LoadX509KeyPair(os.Getenv("TLS_CERT_FILE"), os.Getenv("TLS_KEY_FILE"))
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	caPEM, err := os.ReadFile(os.Getenv("TLS_CA_FILE"))
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return tls.Certificate{}, nil, errors.New("no valid CA certificate found in TLS_CA_FILE")
	}

	return cert, caPool, nil
}

// loadServerTLSConfig builds the policy for INCOMING connections: present our
// cert, and refuse any client that cannot present a certificate signed by the CA.
func loadServerTLSConfig() (*tls.Config, error) {
	cert, caPool, err := loadIdentityAndCA()
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// loadClientTLSConfig builds the policy for OUTGOING connections to peers:
// present our cert when asked, and only accept peers whose cert is signed by
// the CA (and whose cert's SAN matches the hostname in the URL, e.g. "node-b").
func loadClientTLSConfig() (*tls.Config, error) {
	cert, caPool, err := loadIdentityAndCA()
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// newPeerHTTPClient returns the HTTP client used for all node-to-node calls.
func newPeerHTTPClient() (*http.Client, error) {
	tlsConfig, err := loadClientTLSConfig()
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}, nil
}

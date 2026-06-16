package webserver

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	mrand "math/rand"
	"net"
	"os"
	"path"
	"time"
)

var privateKey crypto.Signer
var err error

func init() {
	privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
}

func CreateCert(ca tls.Certificate, host string) (*tls.Certificate, error) {
	// Use the provided CA for certificate generation.
	// Use already parsed Leaf certificate when present.
	x509ca := ca.Leaf
	if x509ca == nil {
		x509ca, err = x509.ParseCertificate(ca.Certificate[0])
		if err != nil {
			return nil, fmt.Errorf("CA ParseCertificate %w", err)
		}
	}

	now := time.Now()
	start := now.Add(-30 * 24 * time.Hour) // -30 days
	end := now.Add(365 * 24 * time.Hour)   // 365 days

	// Always generate a positive int value
	// (Two complement is not enabled when the first bit is 0)
	generated := mrand.Uint64()
	generated >>= 1

	template := x509.Certificate{
		SerialNumber: big.NewInt(int64(generated)),
		Issuer:       x509ca.Subject,
		Subject: pkix.Name{
			CommonName:   host,
			Organization: []string{"GoLancache proxy"},
		},
		NotBefore: start,
		NotAfter:  end,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	ip := net.ParseIP(host)
	if ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host, "*."+host)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, x509ca, privateKey.Public(), ca.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("CreateCertificate %w", err)
	}

	// Save an already parsed leaf certificate to use less CPU
	// when it will be used
	leafCert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, fmt.Errorf("ParseCertificate %w", err)
	}

	certBytes := make([][]byte, 1+len(ca.Certificate))
	certBytes[0] = derBytes
	for i, singleCertBytes := range ca.Certificate {
		certBytes[i+1] = singleCertBytes
	}

	return &tls.Certificate{
		Certificate: certBytes,
		PrivateKey:  privateKey,
		Leaf:        leafCert,
	}, nil
}

func CreateCa(certFile, keyFile string) (tls.Certificate, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:         "github.com/djosmer/go-lancache",
			Organization:       []string{"DjoSmer"},
			OrganizationalUnit: []string{"GoLancache"},
		},

		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(10, 0, 0),

		IsCA:     true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,

		BasicConstraintsValid: true,
	}

	caPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("GenerateKey %w", err)
	}

	KeyBytes, err := x509.MarshalPKCS8PrivateKey(caPrivKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("Get key bytes %w", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, caPrivKey.Public(), caPrivKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("CreateCertificate %w", err)
	}

	if certFile != "" {
		err := exportToPem(certFile, "CERTIFICATE", certBytes)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("Write cert %w", err)
		}
	}
	if keyFile != "" {
		err := exportToPem(keyFile, "PRIVATE KEY", KeyBytes)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("Write key %w", err)
		}
	}

	leafCert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  caPrivKey,
		Leaf:        leafCert,
	}, nil
}

func exportToPem(filepath, pemType string, bytes []byte) error {
	dir := path.Dir(filepath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("Dir %s: %w", dir, err)
	}

	f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		return fmt.Errorf("Open file %s: %w", filepath, err)
	}

	err = pem.Encode(f, &pem.Block{
		Type:  pemType,
		Bytes: bytes,
	})
	if err != nil {
		return fmt.Errorf("Pem Encode %w", err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("Close file %s: %w", filepath, err)
	}

	return nil
}

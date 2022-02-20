/*
 * Tencent is pleased to support the open source community by making TKEStack
 * available.
 *
 * Copyright (C) 2012-2021 Tencent. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use
 * this file except in compliance with the License. You may obtain a copy of the
 * License at
 *
 * https://opensource.org/licenses/Apache-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OF ANY KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations under the License.
 */

package secure

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	rd "math/rand"
	"net"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
	"tkestack.io/kstone-etcd-operator/pkg/controller"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/helpers"
)

const (
	// EtcdCACertSuffix define the suffix of etcd CA certificate
	EtcdCACertSuffix = "-etcd-cacert"
	// EtcdServerCertSuffix define the suffix of etcd server certificate
	EtcdServerCertSuffix = "-etcd-server-cert"
	// EtcdPeerCertSuffix define the suffix of etcd peer certificate
	EtcdPeerCertSuffix = "-etcd-peer-cert"
	// EtcdClientCertSuffix define the suffix of etcd client certificate
	EtcdClientCertSuffix = "-etcd-client-cert"

	// EtcdCACertName if the name of etcd CA certificate
	EtcdCACertName = "ca.pem"
	// EtcdCAKeyName is the name of etcd CA private key
	EtcdCAKeyName = "ca-key.pem"
	// EtcdServerCertName is the name of etcd server certificate
	EtcdServerCertName = "server.pem"
	// EtcdServerKeyName is the name of etcd server private key
	EtcdServerKeyName = "server-key.pem"
	// EtcdPeerCertName is the name of etcd peer certificate
	EtcdPeerCertName = "peer.pem"
	// EtcdPeerKeyName is the name of etcd peer private key
	EtcdPeerKeyName = "peer-key.pem"
	// EtcdClientCertName is the name of etcd client certificate
	EtcdClientCertName = "client.pem"
	// EtcdClientKeyName is the name of etcd client private key
	EtcdClientKeyName = "client-key.pem"
)

// EtcdCertType define the type of etcd cluster certificate
type EtcdCertType string

const (
	// EtcdCACert indicates that the certificate is an etcd CA certificate
	EtcdCACert EtcdCertType = "ca"
	// EtcdServerCert means the certificate is used by etcd server
	EtcdServerCert EtcdCertType = "server"
	// EtcdPeerCert means the certificate is used by etcd peer communication
	EtcdPeerCert EtcdCertType = "peer"
	// EtcdClientCert means the certificate is used by etcd client to communicating with etcd server
	EtcdClientCert EtcdCertType = "client"
)

// NameForEtcdCertSecret return the default secret name according to the specified EtcdCertType: ca, server, peer, client.
func NameForEtcdCertSecret(etcdName string, t EtcdCertType) string {
	switch t {
	case EtcdCACert:
		return etcdName + EtcdCACertSuffix
	case EtcdServerCert:
		return etcdName + EtcdServerCertSuffix
	case EtcdPeerCert:
		return etcdName + EtcdPeerCertSuffix
	case EtcdClientCert:
		return etcdName + EtcdClientCertSuffix
	default:
		return ""
	}
}

// GenerateSelfSignedCACertAndKey generate an self-signed CA and Key
func GenerateSelfSignedCACertAndKey(CN string) ([]byte, []byte, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	caCrt := x509.Certificate{
		SerialNumber: big.NewInt(rd.Int63()),
		Subject: pkix.Name{
			CommonName: CN,
		},
		NotBefore:             now.UTC(),
		NotAfter:              now.AddDate(10, 0, 0).UTC(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCrtBuf, err := x509.CreateCertificate(rand.Reader, &caCrt, &caCrt, caKey.Public(), caKey)
	if err != nil {
		return nil, nil, err
	}
	caCrtByte := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCrtBuf})
	caKeyByte := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caKey)})

	return caCrtByte, caKeyByte, nil
}

// CertConfig is the configuration for generating certificate
type CertConfig struct {
	CommonName   string
	Organization []string

	IsCA bool

	DNSs []string
	IPs  []net.IP

	CACert     []byte
	CAKey      []byte
	PrivateKey []byte

	KeyUsage    x509.KeyUsage
	ExtKeyUsage []x509.ExtKeyUsage

	NotAfter time.Time
}

// GenerateCertAndKey will generate a certificate and private key using the provided CA.
// If the PrivateKey is not provided, we will generate one.
func GenerateCertAndKey(cfg CertConfig) ([]byte, []byte, error) {
	if len(cfg.CAKey) == 0 || len(cfg.CACert) == 0 {
		return nil, nil, errors.New("CAKey and CACert must be provided")
	}
	var privateKey *rsa.PrivateKey
	var err error
	keyBuf := bytes.NewBuffer([]byte(""))
	if len(cfg.PrivateKey) == 0 {
		privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, nil, err
		}
		marshaledKey := x509.MarshalPKCS1PrivateKey(privateKey)
		privateKeyPem := pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: marshaledKey,
		}
		err = pem.Encode(keyBuf, &privateKeyPem)
		if err != nil {
			return nil, nil, errors.Wrap(err, "pem Encode client key error")
		}
	} else {
		keyBlock, _ := pem.Decode(cfg.PrivateKey)
		privateKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, nil, errors.Wrap(err, "x509 ParsePKCS1PrivateKey error")
		}
		keyBuf = bytes.NewBuffer(cfg.PrivateKey)
	}

	caBlock, _ := pem.Decode(cfg.CACert)
	cert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return nil, nil, errors.Wrap(err, "x509 ParseCertificate error")
	}

	caKeyBlock, _ := pem.Decode(cfg.CAKey)
	caPrivate, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, nil, errors.Wrap(err, "x509 ParsePKCS1PrivateKey error")
	}

	notBefore := time.Now().UTC()
	certTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(rd.Int63()),
		NotBefore:             notBefore,
		NotAfter:              cfg.NotAfter.UTC(),
		BasicConstraintsValid: true,
		IsCA:                  cfg.IsCA,
		KeyUsage:              cfg.KeyUsage,
		ExtKeyUsage:           cfg.ExtKeyUsage,
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:    cfg.DNSs,
		IPAddresses: cfg.IPs,
	}

	certEncode, err := x509.CreateCertificate(rand.Reader, &certTemplate, cert, &privateKey.PublicKey, caPrivate)
	if err != nil {
		return nil, nil, errors.Wrap(err, "x509 CreateCertificate error")
	}

	var certBuf bytes.Buffer
	err = pem.Encode(&certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certEncode})
	if err != nil {
		return nil, nil, errors.Wrap(err, "pem Encode certificate error")
	}
	return certBuf.Bytes(), keyBuf.Bytes(), nil
}

// NewEtcdCASecret return a secret which contains a self-signed etcd ca cert and key
func NewEtcdCASecret(e *v1alpha1.EtcdCluster) (*corev1.Secret, error) {
	caCrt, caKey, err := GenerateSelfSignedCACertAndKey(e.Name)
	if err != nil {
		return nil, err
	}
	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: e.Namespace,
			Name:      NameForEtcdCertSecret(e.Name, EtcdCACert),
		},
		Data: map[string][]byte{
			EtcdCACertName: caCrt,
			EtcdCAKeyName:  caKey,
		},
	}
	controller.SetEtcdOwnerReference(&secret.ObjectMeta, e)

	return secret, nil
}

// defaultDNSsForEtcdServer return the dns name list which should be signed in etcd cluster's server certificate
func defaultDNSsForEtcdServer(e *v1alpha1.EtcdCluster) []string {
	as := helpers.NameForAccessService(e)
	hs := helpers.NameForHeadlessService(e)
	return []string{
		fmt.Sprintf("%s.%s.svc.%s", as, e.Namespace, helpers.ClusterDomain(e)),
		fmt.Sprintf("%s.%s.svc.%s", hs, e.Namespace, helpers.ClusterDomain(e)),
		fmt.Sprintf("*.%s.%s.svc.%s", hs, e.Namespace, helpers.ClusterDomain(e)),
	}
}

// NewEtcdCertSecret returns a new etcd certificate secret based on the provided etcd cluster and certificate type
func NewEtcdCertSecret(e *v1alpha1.EtcdCluster, caSecret *corev1.Secret, t EtcdCertType) (*corev1.Secret, error) {
	switch t {
	case EtcdPeerCert:
		return NewEtcdPeerCertSecret(e, caSecret)
	case EtcdServerCert:
		return NewEtcdServerCertSecret(e, caSecret)
	case EtcdClientCert:
		return NewEtcdClientCertSecret(e, caSecret)
	default:
		return nil, errors.Errorf("unsupport cert type: %s", t)
	}
}

// NewEtcdServerCertSecret will generate an new etcd server certificate secret based on the etcd cluster's configuration
func NewEtcdServerCertSecret(e *v1alpha1.EtcdCluster, caSecret *corev1.Secret) (*corev1.Secret, error) {
	dnss := defaultDNSsForEtcdServer(e)
	dnss = append(dnss, e.Spec.Secure.TLS.AutoTLSCert.ExtraServerCertSANs...)

	var ips []net.IP
	for _, i := range e.Status.LoadBalancer.Ingress {
		ips = append(ips, net.ParseIP(i.IP))
	}

	cert, key, err := GenerateCertAndKey(CertConfig{
		CommonName:  "root",
		DNSs:        dnss,
		IPs:         ips,
		CACert:      caSecret.Data[EtcdCACertName],
		CAKey:       caSecret.Data[EtcdCAKeyName],
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		NotAfter:    time.Now().AddDate(10, 0, 0).UTC(),
	})
	if err != nil {
		return nil, err
	}
	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: e.Namespace,
			Name:      NameForEtcdCertSecret(e.Name, EtcdServerCert),
		},
		Data: map[string][]byte{
			EtcdCACertName:     caSecret.Data[EtcdCACertName],
			EtcdServerCertName: cert,
			EtcdServerKeyName:  key,
		},
	}
	controller.SetEtcdOwnerReference(&secret.ObjectMeta, e)

	return secret, nil
}

// NewEtcdPeerCertSecret will generate an etcd peer certificate for etcd communication
func NewEtcdPeerCertSecret(e *v1alpha1.EtcdCluster, caSecret *corev1.Secret) (*corev1.Secret, error) {
	cert, key, err := GenerateCertAndKey(CertConfig{
		CommonName: "root",
		// because nslookup $IP will return a random service domain, etcd will use checkCertSAN to validate the ip and domain matched,
		// so we need sign all service domain here
		DNSs:        defaultDNSsForEtcdServer(e),
		CACert:      caSecret.Data[EtcdCACertName],
		CAKey:       caSecret.Data[EtcdCAKeyName],
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		NotAfter:    time.Now().AddDate(10, 0, 0).UTC(),
	})
	if err != nil {
		return nil, err
	}
	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: e.Namespace,
			Name:      NameForEtcdCertSecret(e.Name, EtcdPeerCert),
		},
		Data: map[string][]byte{
			EtcdCACertName:   caSecret.Data[EtcdCACertName],
			EtcdPeerCertName: cert,
			EtcdPeerKeyName:  key,
		},
	}
	controller.SetEtcdOwnerReference(&secret.ObjectMeta, e)

	return secret, nil
}

// NewEtcdClientCertSecret will generate a client certificate for connecting to etcd cluster
func NewEtcdClientCertSecret(e *v1alpha1.EtcdCluster, caSecret *corev1.Secret) (*corev1.Secret, error) {
	cert, key, err := GenerateCertAndKey(CertConfig{
		CommonName:  "root",
		CACert:      caSecret.Data[EtcdCACertName],
		CAKey:       caSecret.Data[EtcdCAKeyName],
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		NotAfter:    time.Now().AddDate(10, 0, 0).UTC(),
	})
	if err != nil {
		return nil, err
	}
	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: e.Namespace,
			Name:      NameForEtcdCertSecret(e.Name, EtcdClientCert),
		},
		Data: map[string][]byte{
			EtcdCACertName:     caSecret.Data[EtcdCACertName],
			EtcdClientCertName: cert,
			EtcdClientKeyName:  key,
		},
	}
	controller.SetEtcdOwnerReference(&secret.ObjectMeta, e)

	return secret, nil
}

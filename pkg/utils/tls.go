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

package utils

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// NewTLSFromString constructs the tls config from the provided certificate string
func NewTLSFromString(caCert, cert, key string) (*tls.Config, error) {
	tlsCert, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err != nil {
		return nil, err
	}

	cfg := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS10,
	}

	// decode ca cert
	block, _ := pem.Decode([]byte(caCert))
	if block == nil {
		return nil, fmt.Errorf("NewTransportFromString parse failed")
	}
	caCertPemDecoded, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(caCertPemDecoded)
	cfg.RootCAs = certPool
	cfg.InsecureSkipVerify = true

	return cfg, nil
}

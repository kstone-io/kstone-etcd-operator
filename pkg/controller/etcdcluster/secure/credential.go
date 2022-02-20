/*
 * Tencent is pleased to support the open source community by making TKEStack
 * available.
 *
 * Copyright (C) 2012-2022 Tencent. All Rights Reserved.
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
	corev1 "k8s.io/api/core/v1"
)

// Credential is the credential to access etcd cluster
type Credential struct {
	// Username is a username for authentication.
	Username string `json:"username"`
	// Password is a password for authentication.
	Password string `json:"password"`
}

// CredentialFromSecret get the credential used by operator to access etcd cluster,
// return nil if the credential is invalid
func CredentialFromSecret(secret *corev1.Secret) *Credential {
	cred := &Credential{}
	username, ok := secret.Data["username"]
	if !ok {
		return nil
	}
	cred.Username = string(username)

	password, ok := secret.Data["password"]
	if !ok {
		return nil
	}
	cred.Password = string(password)

	return cred
}

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

package loadbalancer

import (
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
)

// ProviderIdentifierName defines the load balancer provider used to expose etcd cluster access
const ProviderIdentifierName = "etcdcluster.etcd.tkestack.io/loadbalancer-provider-name"

// Provider define loadbalancer provider interface
type Provider interface {
	ApplyTo(annotations map[string]string, service *v1.Service) error
	NeedUpdate(annotations map[string]string, service *v1.Service) bool
}

// Factory defines loadbalancer provider factory
type Factory func() (Provider, error)

var (
	mutex     sync.Mutex
	providers = make(map[string]Factory)
)

// RegisterLoadBalancerProvider register a loadbalancer provider, the provider is a factory
func RegisterLoadBalancerProvider(name string, provider Factory) {
	mutex.Lock()
	defer mutex.Unlock()
	if _, found := providers[name]; found {
		panic(fmt.Sprintf("provider %s already registered", name))
	}
	providers[name] = provider
}

// GetLoadBalancerProvider return the specified loadbalancer provider
func GetLoadBalancerProvider(name string) (Provider, error) {
	mutex.Lock()
	defer mutex.Unlock()
	f, found := providers[name]
	if !found {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return f()
}

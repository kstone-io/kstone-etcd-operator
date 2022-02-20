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

package none

import (
	v1 "k8s.io/api/core/v1"

	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/service/loadbalancer"
)

const providerName = ""

type none struct {
}

func init() {
	loadbalancer.RegisterLoadBalancerProvider(providerName, func() (loadbalancer.Provider, error) {
		return NewNoneLoadBalancerProvider(), nil
	})
}

// NewNoneLoadBalancerProvider returns the default loadbalancer provider that uses ClusterIP as the loadbalancer
func NewNoneLoadBalancerProvider() loadbalancer.Provider {
	return &none{}
}

// NeedUpdate indicate if the service need to update
func (t *none) NeedUpdate(annotations map[string]string, svc *v1.Service) bool {
	return svc.Spec.Type != v1.ServiceTypeClusterIP
}

// ApplyTo  apply the configuration in annotations to the service
func (t *none) ApplyTo(annotations map[string]string, svc *v1.Service) error {
	svc.Spec.Type = v1.ServiceTypeClusterIP
	return nil
}

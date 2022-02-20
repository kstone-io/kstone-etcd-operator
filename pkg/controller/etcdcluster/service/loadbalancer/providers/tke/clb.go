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

package tke

import (
	v1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"

	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/service/loadbalancer"
)

const providerName = "tke"

// AnnotationLoadBalancerInternalSubnetID determines which subnet the Tencent Cloud loadbalancer will be allocated to
const AnnotationLoadBalancerInternalSubnetID = "service.kubernetes.io/qcloud-loadbalancer-internal-subnetid"

type tke struct {
}

func init() {
	loadbalancer.RegisterLoadBalancerProvider(providerName, func() (loadbalancer.Provider, error) {
		return NewTkeLoadBalancerProvider(), nil
	})
}

// NewTkeLoadBalancerProvider return tke loadbalancer provider
func NewTkeLoadBalancerProvider() loadbalancer.Provider {
	return &tke{}
}

// NeedUpdate indicate if the service need to update
func (t *tke) NeedUpdate(annotations map[string]string, svc *v1.Service) bool {
	if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
		return true
	}

	if expect, ok := annotations[AnnotationLoadBalancerInternalSubnetID]; ok {
		if svc.Annotations == nil {
			return true
		}
		if actual, ok := svc.Annotations[AnnotationLoadBalancerInternalSubnetID]; !ok || expect != actual {
			return true
		}
	} else {
		if svc.Annotations != nil && svc.Annotations[AnnotationLoadBalancerInternalSubnetID] != "" {
			return true
		}
	}

	return false
}

// ApplyTo apply the configuration in annotations to the service
func (t *tke) ApplyTo(annotations map[string]string, svc *v1.Service) error {

	klog.Infof("before update annotations: %v, service annotations: %v", annotations, svc.Annotations)

	svc.Spec.Type = v1.ServiceTypeLoadBalancer
	if _, ok := annotations[AnnotationLoadBalancerInternalSubnetID]; ok {
		if svc.Annotations == nil {
			svc.Annotations = make(map[string]string)
		}
		svc.Annotations[AnnotationLoadBalancerInternalSubnetID] = annotations[AnnotationLoadBalancerInternalSubnetID]
	} else {
		delete(svc.Annotations, AnnotationLoadBalancerInternalSubnetID)
	}
	klog.Infof("after update annotations: %v, service annotations: %v", annotations, svc.Annotations)

	return nil
}

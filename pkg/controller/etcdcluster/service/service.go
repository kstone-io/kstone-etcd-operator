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

package service

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
	"tkestack.io/kstone-etcd-operator/pkg/controller"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/constants"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/helpers"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/service/loadbalancer"
)

// NewEtcdHeadlessService will generate a headless service for etcd node internal access and discovery
func NewEtcdHeadlessService(e *v1alpha1.EtcdCluster) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Namespace: e.Namespace,
			Name:      helpers.NameForHeadlessService(e),
			Labels: map[string]string{
				constants.LabelEtcdClusterNameSelector: e.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			ClusterIP:                corev1.ClusterIPNone,
			PublishNotReadyAddresses: true,
			Ports: []corev1.ServicePort{
				{
					Name:       constants.EtcdPeerPortName,
					Port:       constants.EtcdDefaultPeerPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(constants.EtcdDefaultPeerPort),
				},
			},
			Selector: map[string]string{
				constants.LabelEtcdClusterNameSelector: e.Name,
			},
		},
	}

	controller.SetEtcdOwnerReference(&svc.ObjectMeta, e)

	return svc, nil
}

// NewEtcdAccessService generate a service for client accessing etcd cluster, default type is ClusterIP.
// you can add an LoadBalancer provider annotation on the etcdcluster if you need a LoadBalancer type service
func NewEtcdAccessService(e *v1alpha1.EtcdCluster) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Namespace: e.Namespace,
			Name:      helpers.NameForAccessService(e),
		},
	}
	if err := applyAccessService(e, svc); err != nil {
		return nil, err
	}

	return svc, nil
}

func applyAccessService(e *v1alpha1.EtcdCluster, svc *corev1.Service) error {
	// apply labels
	if svc.Labels == nil {
		svc.Labels = make(map[string]string)
	}
	svc.Labels[constants.LabelEtcdClusterNameSelector] = e.Name

	// apply selector
	if svc.Spec.Selector == nil {
		svc.Spec.Selector = make(map[string]string)
	}
	svc.Spec.Selector[constants.LabelEtcdClusterNameSelector] = e.Name
	svc.Spec.Selector[constants.LabelEtcdVotingMemberSelector] = "true"

	// apply client port
	clientPort := corev1.ServicePort{
		Name:       constants.EtcdClientPortName,
		Port:       constants.EtcdDefaultClientPort,
		Protocol:   corev1.ProtocolTCP,
		TargetPort: intstr.FromInt(constants.EtcdDefaultClientPort),
	}
	if e.Spec.ClientPort != nil {
		clientPort.Port = *e.Spec.ClientPort
		clientPort.TargetPort = intstr.FromInt(int(*e.Spec.ClientPort))
	}

	portMap := make(map[string]int)
	for index, port := range svc.Spec.Ports {
		portMap[port.Name] = index
	}
	if index, ok := portMap[clientPort.Name]; ok {
		svc.Spec.Ports[index].Port = clientPort.Port
		svc.Spec.Ports[index].Protocol = clientPort.Protocol
		svc.Spec.Ports[index].TargetPort = clientPort.TargetPort
	} else {
		svc.Spec.Ports = append(svc.Spec.Ports, clientPort)
	}

	// apply loadbalancer
	provider, err := loadbalancer.GetLoadBalancerProvider(e.Annotations[loadbalancer.ProviderIdentifierName])
	if err != nil {
		return err
	}
	if err := provider.ApplyTo(e.Annotations, svc); err != nil {
		return err
	}

	// set owner
	controller.SetEtcdOwnerReference(&svc.ObjectMeta, e)

	return nil
}

// GetUpdateAccessService return an updated service when etcdcluster's service need to update
func GetUpdateAccessService(e *v1alpha1.EtcdCluster, old *corev1.Service) (*corev1.Service, error) {
	updated := old.DeepCopy()
	if err := applyAccessService(e, updated); err != nil {
		return nil, err
	}
	return updated, nil
}

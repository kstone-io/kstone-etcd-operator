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

package helpers

import (
	"fmt"

	"tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/constants"
)

// NameForHeadlessService returns the headless service name used by etcdcluster
func NameForHeadlessService(e *v1alpha1.EtcdCluster) string {
	return e.Name + constants.EtcdHeadlessServiceSuffix
}

// NameForAccessService returns the service name provided to clients connecting etcdcluster
func NameForAccessService(e *v1alpha1.EtcdCluster) string {
	return e.Name + constants.EtcdAccessServiceSuffix
}

// PVCNameForEtcd return the name of persistent volume used by etcd cluster
func PVCNameForEtcd(e *v1alpha1.EtcdCluster, index int32) string {
	return fmt.Sprintf("%s-%s-%d", constants.EtcdDataVolumeName, NameForTApp(e), index)
}

// NameForTApp generate the etcd cluster's TApp name
func NameForTApp(e *v1alpha1.EtcdCluster) string {
	return e.Name + "-etcd"
}

// IsClientSecure return true if the etcd cluster use server TLS configuration
func IsClientSecure(e *v1alpha1.EtcdCluster) bool {
	if e.Spec.Secure == nil || e.Spec.Secure.TLS == nil {
		return false
	}
	tlsConfig := e.Spec.Secure.TLS
	if tlsConfig.ExternalCerts != nil && tlsConfig.ExternalCerts.ServerSecret != "" {
		return true
	}
	if tlsConfig.AutoTLSCert != nil && tlsConfig.AutoTLSCert.AutoGenerateServerCert {
		return true
	}
	return false
}

// IsPeerSecure return true if the etcd cluster use peer TLS configuration
func IsPeerSecure(e *v1alpha1.EtcdCluster) bool {
	if e.Spec.Secure == nil || e.Spec.Secure.TLS == nil {
		return false
	}
	tlsConfig := e.Spec.Secure.TLS
	if tlsConfig.ExternalCerts != nil && tlsConfig.ExternalCerts.PeerSecret != "" {
		return true
	}
	if tlsConfig.AutoTLSCert != nil && tlsConfig.AutoTLSCert.AutoGeneratePeerCert {
		return true
	}
	return false
}

// protocolScheme return https if the endpoint is secure
func protocolScheme(isSecure bool) string {
	if isSecure {
		return "https"
	}
	return "http"
}

// SchemeForClient returns https if the etcd cluster specifies a server certificate, otherwise returns http
func SchemeForClient(e *v1alpha1.EtcdCluster) string {
	return protocolScheme(IsClientSecure(e))
}

// SchemeForPeer returns https if the etcd cluster specifies a peer certificate, otherwise returns http
func SchemeForPeer(e *v1alpha1.EtcdCluster) string {
	return protocolScheme(IsPeerSecure(e))
}

// AccessURL return the etcd cluster's advertise URL, you can use this URL to access the etcd cluster
func AccessURL(e *v1alpha1.EtcdCluster) string {
	return fmt.Sprintf("%s://%s:%d", SchemeForClient(e), fmt.Sprintf("%s.%s.svc.%s", NameForAccessService(e), e.Namespace, ClusterDomain(e)), func() int32 {
		if e.Spec.ClientPort != nil {
			return *e.Spec.ClientPort
		}
		return constants.EtcdDefaultClientPort
	}())
}

// AdvertiseClientURLs return the etcd cluster's URL which for client access
func AdvertiseClientURLs(e *v1alpha1.EtcdCluster) []string {
	clientPort := constants.EtcdDefaultClientPort
	if e.Spec.ClientPort != nil {
		clientPort = int(*e.Spec.ClientPort)
	}
	return []string{
		fmt.Sprintf("%s://%s:%d", SchemeForClient(e), fmt.Sprintf("%s.%s.svc.%s", NameForAccessService(e), e.Namespace, ClusterDomain(e)), clientPort),
	}
}

// PeerNodeURL return the URL where etcd nodes communicate with each other
func PeerNodeURL(e *v1alpha1.EtcdCluster, index int32) string {
	return fmt.Sprintf("%s://%s:%d", SchemeForPeer(e), fmt.Sprintf("%s-%d.%s.%s.svc.%s", NameForTApp(e), index, NameForHeadlessService(e), e.Namespace, ClusterDomain(e)), constants.EtcdDefaultPeerPort)
}

// NodeAccessEndpoint return the endpoint address for the specified etcd node
func NodeAccessEndpoint(e *v1alpha1.EtcdCluster, index int32) string {
	clientPort := constants.EtcdDefaultClientPort
	if e.Spec.ClientPort != nil {
		clientPort = int(*e.Spec.ClientPort)
	}
	return fmt.Sprintf("%s://%s:%d", SchemeForClient(e), fmt.Sprintf("%s-%d.%s.%s.svc.%s", NameForTApp(e), index, NameForHeadlessService(e), e.Namespace, ClusterDomain(e)), clientPort)
}

// CanOperate indicate whether operator can control this etcdcluster
func CanOperate(e *v1alpha1.EtcdCluster) bool {
	if e.Labels != nil && e.Labels[constants.LabelOperatorControlPaused] == "true" {
		return false
	}
	return true
}

// ClusterDomain return the cluster domain used by etcd service access
func ClusterDomain(e *v1alpha1.EtcdCluster) string {
	if e.Annotations != nil && e.Annotations[constants.AnnotationClusterDomain] != "" {
		return e.Annotations[constants.AnnotationClusterDomain]
	}
	return constants.DefaultClusterDomain
}

// ImageForEtcd returns the image address based on etcd version and repository
func ImageForEtcd(e *v1alpha1.EtcdCluster) string {
	version := v1alpha1.DefaultEtcdVersion
	if e.Spec.Version != "" {
		version = e.Spec.Version
	}

	repo := v1alpha1.DefaultRepository

	if e.Spec.Repository != "" {
		repo = e.Spec.Repository
	}

	return fmt.Sprintf("%s:v%s", repo, version)
}

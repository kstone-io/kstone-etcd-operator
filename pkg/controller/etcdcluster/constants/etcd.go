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

package constants

const (
	// LabelEtcdClusterNameSelector define the etcd cluster name selector
	LabelEtcdClusterNameSelector = "etcdcluster.etcd.tkestack.io/cluster-name"
	// LabelEtcdVotingMemberSelector define the role of an etcd node, only voting members can be accessed by the client
	LabelEtcdVotingMemberSelector = "etcdcluster.etcd.tkestack.io/voting-member"

	// LabelOperatorControlPaused indicates whether etcdcluster is controlled by kstone etcd operator
	LabelOperatorControlPaused = "etcdcluster.etcd.tkestack.io/control-paused"

	// DefaultClusterDomain define the default cluster domain, which is cluster.local
	DefaultClusterDomain = "cluster.local"
	// AnnotationClusterDomain defines the cluster domain used by etcd service access
	AnnotationClusterDomain = "etcdcluster.etcd.tkestack.io/cluster-domain"

	// EtcdVersionGlobalConfigName is the name of etcd version global config
	EtcdVersionGlobalConfigName = "etcd-version-global-config"

	// EtcdDataVolumeName is the name of etcd data volume
	EtcdDataVolumeName = "etcd-data"
	// EtcdDataVolumeMountPath is the path which etcd data volume will be mounted
	EtcdDataVolumeMountPath = "/var/lib/etcd"

	// EtcdServerTLSVolumeName is the name of etcd server certificate volume
	EtcdServerTLSVolumeName = "etcd-server-tls"
	// EtcdServerTLSMountPath is the path which etcd server certificate will be mounted and stored
	EtcdServerTLSMountPath = "/etc/etcd/pki/server"
	// EtcdPeerTLSVolumeName is the name of etcd peer certificate volume
	EtcdPeerTLSVolumeName = "etcd-peer-tls"
	// EtcdPeerTLSMountPath is the path which etcd peer certificate will be mounted and stored
	EtcdPeerTLSMountPath = "/etc/etcd/pki/peer"
	// EtcdClientTLSVolumeName is the name of etcd client certificate volume
	EtcdClientTLSVolumeName = "etcd-client-tls"
	// EtcdClientTLSMountPath is the path which etcd client certificate will be mounted and stored
	EtcdClientTLSMountPath = "/etc/etcd/pki/client"
)

const (
	// EtcdHeadlessServiceSuffix defines the suffix for headless services used by etcd cluster
	EtcdHeadlessServiceSuffix = "-etcd-headless"
	// EtcdAccessServiceSuffix defines the suffix for access services used by etcd cluster
	EtcdAccessServiceSuffix = "-etcd"
	// EtcdPeerPortName is the peer port name used in etcd cluster's service port
	EtcdPeerPortName = "peer"
	// EtcdDefaultPeerPort define the default etcd peer port, which is 2380
	EtcdDefaultPeerPort = 2380
	// EtcdClientPortName is the client port name used in etcd cluster's service port
	EtcdClientPortName = "client"
	// EtcdDefaultClientPort define the default etcd client port, which is 2379
	EtcdDefaultClientPort = 2379
)

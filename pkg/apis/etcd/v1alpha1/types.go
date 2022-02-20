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

// +k8s:deepcopy-gen=package
// +groupName=etcd.tkestack.io

// Package v1alpha1 is the v1alpha1 version of the API.
package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdCluster is a specification for a EtcdCluster resource
// +kubebuilder:resource:shortName=etcd
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SIZE",type=string,JSONPath=`.spec.size`
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
type EtcdCluster struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the EtcdCluster.
	// +optional
	Spec EtcdClusterSpec `json:"spec,omitempty"`
	// Most recently observed status of the EtcdCluster.
	// +optional
	Status EtcdClusterStatus `json:"status,omitempty"`
}

// ProviderType define the type of etcd cluster's provider
type ProviderType string

const (
	// ProviderTApp will use the tapp controller to manage the etcd node
	ProviderTApp ProviderType = "TApp"
)

const (
	// DefaultRepository define the default etcd image repository
	DefaultRepository = "quay.io/coreos/etcd"
	// DefaultEtcdVersion define the default etcd version
	DefaultEtcdVersion = "3.4.13"
)

// EtcdClusterSpec is the spec for a EtcdCluster resources
type EtcdClusterSpec struct {
	// You can choose different Provider as the carrier for etcd to run
	Provider ProviderType `json:"provider,omitempty"`
	// Size is the etcd cluster's node count, contains voter member and learner
	Size int32 `json:"size"`
	// Version is the etcd cluster version, which follow the semantic versioning specification, like v3.4.13
	Version string `json:"version"`
	// Repository is the name of the repository that hosts etcd container images.
	// By default, it is `quay.io/coreos/etcd`.
	Repository string `json:"repository,omitempty"`
	// ClientPort is the client port which using to access an etcd cluster
	// +optional
	ClientPort *int32 `json:"clientPort,omitempty"`
	// Template is the etcd cluster's deployment template
	Template EtcdTemplateSpec `json:"template,omitempty"`
	// Secure contains the etcd cluster's tls and authentication configuration
	Secure *SecureConfig `json:"secure,omitempty"`
	// Learners contains the sequence number of the etcd node which will be created as a learner node.
	// You can remove the corresponding sequence number when you want to promote learner to member
	Learners []string `json:"learners,omitempty"`
}

// CertConfig contains all external certificate which etcd will use
type CertConfig struct {
	MemberSecret `json:",inline"`
	// ClientSecret contains the certificate and key which etcd use, including ca.pem, client.crt, client.key
	ClientSecret string `json:"clientSecret"`
}

// MemberSecret contains the server and peer certificate secret used by etcd cluster communication
type MemberSecret struct {
	// ServerSecret contains the certificate and key which etcd use, including ca.pem, server.crt, server.key
	ServerSecret string `json:"serverSecret"`
	// PeerSecret contains the certificate and key which etcd use, including ca.pem, peer.crt, peer.key
	PeerSecret string `json:"peerSecret"`
}

// AutoGenerateCertConfig contains the cert generating option of etcd operator
type AutoGenerateCertConfig struct {
	AutoGenerateServerCert bool `json:"autoGenerateServerCert,omitempty"`
	AutoGeneratePeerCert   bool `json:"autoGeneratePeerCert,omitempty"`
	AutoGenerateClientCert bool `json:"autoGenerateClientCert,omitempty"`
	// you can use ExternalCASecret to signing the cert which etcd use,
	// if empty, etcd operator will generate a self-signed certificate
	ExternalCASecret string `json:"externalCASecret,omitempty"`
	// ExtraServerCertSANs sets extra Subject Alternative Names for the etcd server signing cert.
	ExtraServerCertSANs []string `json:"extraServerCertSANs,omitempty"`
}

// TLSConfig is the etcd cluster tls certificate configuration,
// you can specify an external certificate secret, or let the operator generate cert
type TLSConfig struct {
	// ExternalCerts means using an extra cert
	ExternalCerts *CertConfig `json:"externalCerts,omitempty"`
	// AutoTLSCert contains the cert generating option of etcd operator
	AutoTLSCert *AutoGenerateCertConfig `json:"autoTLSCert,omitempty"`
}

// AuthConfig is the etcd cluster authorization configuration
type AuthConfig struct {
	// CredentialSecretRef is a secret reference which contains username and password
	// +optional
	CredentialSecretRef *v1.SecretReference `json:"credentialSecret,omitempty"`
}

// SecureConfig is the secure configurations of an etcd cluster
type SecureConfig struct {
	// TLS represents the etcd whether to use https protocol to serve
	TLS *TLSConfig `json:"tls,omitempty"`
	// etcd authorization configuration
	Auth *AuthConfig `json:"auth,omitempty"`
}

// EtcdTemplateSpec describes the attributes that an etcd node is created with.
type EtcdTemplateSpec struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// ExtraArgs will be directly used as extra parameters to start etcd
	ExtraArgs []string `json:"extraArgs,omitempty"`

	// List of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Env []v1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// If specified, the etcd node scheduling constraints
	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`
	// If specified, the pod's tolerations.
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// TopologySpreadConstraints describes how a group of pods ought to spread across topology
	// domains. Scheduler will schedule pods in a way which abides by the constraints.
	// This field is only honored by clusters that enable the EvenPodsSpread feature.
	// All topologySpreadConstraints are ANDed.
	// +optional
	// +patchMergeKey=topologyKey
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=topologyKey
	// +listMapKey=whenUnsatisfiable
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty" patchStrategy:"merge" patchMergeKey:"topologyKey"`

	// PersistentVolumeClaimSpec is a list of claims that pods are allowed to reference.
	PersistentVolumeClaimSpec *v1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimSpec,omitempty"`

	// Compute Resources required by each etcd node.
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
}

// EtcdMemberRole define the role type of etcd node
type EtcdMemberRole string

const (
	// EtcdMemberLeader means current etcd node is leader
	EtcdMemberLeader EtcdMemberRole = "Leader"
	// EtcdMemberFollower means current etcd node is follower
	EtcdMemberFollower EtcdMemberRole = "Follower"
	// EtcdMemberLearner means current etcd node is learner
	EtcdMemberLearner EtcdMemberRole = "Learner"
)

// EtcdMemberStatus define the status of an etcd cluster member
type EtcdMemberStatus string

const (
	// EtcdMemberStatusUnStarted means the etcd node has been registered to the etcd cluster, but not started
	EtcdMemberStatusUnStarted EtcdMemberStatus = "UnStarted"
	// EtcdMemberStatusRunning means the etcd node is running and health
	EtcdMemberStatusRunning EtcdMemberStatus = "Running"
	// EtcdMemberStatusUnknown means the etcd node status is unknown, possibly due to network isolation or node downtime
	EtcdMemberStatusUnknown EtcdMemberStatus = "Unknown"
	// EtcdMemberStatusUnHealthy means the etcd node is unhealthy
	EtcdMemberStatusUnHealthy EtcdMemberStatus = "UnHealthy"
)

// EtcdMember describe the status of an etcd cluster member node
type EtcdMember struct {
	// +kubebuilder:validation:Type=number
	// +kubebuilder:validation:Format=uint64
	ID uint64 `json:"id"`
	// +kubebuilder:validation:Optional
	Name string `json:"name"`
	// +kubebuilder:validation:Optional
	Role EtcdMemberRole `json:"role"`
	// +kubebuilder:validation:Optional
	Version string `json:"version"`
	// +kubebuilder:validation:Optional
	Endpoint string `json:"endpoint,omitempty"`
	// +kubebuilder:validation:Optional
	Status EtcdMemberStatus `json:"status"`
	// +kubebuilder:validation:Optional
	ClientURLs []string `json:"clientURLs"`
}

// EtcdClusterStatus is the status for a EtcdCluster resource
type EtcdClusterStatus struct {
	Members              []EtcdMember           `json:"members,omitempty"`
	ClientCertSecretName string                 `json:"clientCertSecretName,omitempty"`
	Conditions           []EtcdClusterCondition `json:"conditions,omitempty"`
	LoadBalancer         v1.LoadBalancerStatus  `json:"loadBalancer,omitempty"`
	Phase                EtcdClusterPhase       `json:"phase,omitempty"`
	ControlPaused        bool                   `json:"controlPaused,omitempty"`
}

// EtcdClusterPhase is a label for the condition of an etcd cluster at the current time.
type EtcdClusterPhase string

const (
	// ClusterPending means the etcdcluster has been accepted by the system, but has not been started creating.
	ClusterPending EtcdClusterPhase = ""
	// ClusterCreating means the cluster is creating
	ClusterCreating EtcdClusterPhase = "Creating"
	// ClusterRunning means the etcd cluster has been started and can serve clients.
	ClusterRunning EtcdClusterPhase = "Running"
	// ClusterUpdating means the etcd cluster is updating config
	ClusterUpdating EtcdClusterPhase = "Updating"
	// ClusterUnknown means that for some reason the state of the etcdcluster couldn't be obtained.
	ClusterUnknown EtcdClusterPhase = "Unknown"
	// ClusterFailed means that the cluster is in a failed state and can't be recovered because of some reason.
	ClusterFailed EtcdClusterPhase = "Failed"
	// ClusterUnHealthy means that the cluster is in an unhealthy status and currently unable to provide services
	ClusterUnHealthy EtcdClusterPhase = "UnHealthy"
)

// EtcdClusterConditionType is a valid value for EtcdClusterCondition.Type
type EtcdClusterConditionType string

// These are valid conditions of an etcdcluster
const (
	// ClusterAvailable represents if the etcd cluster status is healthy.
	ClusterAvailable EtcdClusterConditionType = "Available"
	// ClusterUpgrading means the etcd cluster is in upgrading progress.
	ClusterUpgrading EtcdClusterConditionType = "Upgrading"
	// ClusterScaling represents that the etcd cluster size is scaling up or scaling down.
	ClusterScaling EtcdClusterConditionType = "Scaling"
	// ClusterRecovering indicates whether etcd cluster is recovering from etcd backup.
	ClusterRecovering EtcdClusterConditionType = "Recovering"
)

// EtcdClusterCondition contains details for the current condition of this etcdcluster.
type EtcdClusterCondition struct {
	// Type of etcdcluster condition.
	Type EtcdClusterConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// Last time we got an update on a given condition.
	// +optional
	LastHeartbeatTime metav1.Time `json:"lastHeartbeatTime,omitempty"`
	// Last time the condition transit from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// (brief) reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// Human readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdClusterList is a list of EtcdCluster resources
type EtcdClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// List of etcdclusters
	Items []EtcdCluster `json:"items"`
}

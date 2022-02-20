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

package etcdcluster

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/helpers"
)

const (
	checkDNSScriptTemplate = `TIMEOUT_READY=15
echo ${POD_DNS_NAME}
while ( ! nslookup ${POD_DNS_NAME} )
do
  # If TIMEOUT_READY is 0 we should never time out and exit 
  TIMEOUT_READY=$(( TIMEOUT_READY-1 ))
  if [ $TIMEOUT_READY -eq 0 ];
  then
    echo "Timed out waiting for DNS entry"
  exit 1
  fi
sleep 1
done`
)

const (
	defaultBusyboxImage = "busybox:1.28.0-glibc"
	// AnnotationInitImage is an init image annotation, you can specify this annotation to replace the default init image
	AnnotationInitImage = "etcdcluster.etcd.tkestack.io/init-image"
)

func getInitImage(e *v1alpha1.EtcdCluster) string {
	if image, ok := e.Annotations[AnnotationInitImage]; ok {
		return image
	}
	return defaultBusyboxImage
}

// NewEtcdInitContainer returns the init container used to start etcd node
func NewEtcdInitContainer(e *v1alpha1.EtcdCluster) corev1.Container {
	initEnv := []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "POD_DNS_NAME",
			Value: fmt.Sprintf("$(POD_NAME).%s.%s.svc.%s", helpers.NameForHeadlessService(e), e.Namespace, helpers.ClusterDomain(e)),
		},
	}
	return corev1.Container{
		Image:   getInitImage(e),
		Name:    "check-dns",
		Command: []string{"/bin/sh", "-ec", checkDNSScriptTemplate},
		Env:     initEnv,
	}
}

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

package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	"tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

// SetEtcdOwnerReference is responsible for setting the object's owner to the specified EtcdCluster
func SetEtcdOwnerReference(meta *metav1.ObjectMeta, e *v1alpha1.EtcdCluster) {
	ref := metav1.OwnerReference{
		APIVersion:         v1alpha1.SchemeGroupVersion.String(),
		Kind:               "EtcdCluster",
		Name:               e.Name,
		UID:                e.UID,
		Controller:         pointer.BoolPtr(true),
		BlockOwnerDeletion: pointer.BoolPtr(true),
	}

	found := false

	for idx, r := range meta.OwnerReferences {
		if r.Name == ref.Name && r.APIVersion == v1alpha1.SchemeGroupVersion.String() && r.Kind == "EtcdCluster" {
			found = true
			meta.OwnerReferences[idx] = ref
			break
		}
	}

	if !found {
		meta.OwnerReferences = append(meta.OwnerReferences, ref)
	}
}

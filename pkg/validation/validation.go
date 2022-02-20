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

package validation

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
)

// Validate validate an etcd cluster
func Validate(e *v1alpha1.EtcdCluster) field.ErrorList {
	etcd := e.DeepCopy()
	allErrs := field.ErrorList{}

	fldPath := field.NewPath("spec")

	if etcd.Spec.Size < 1 || etcd.Spec.Size > 7 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("size"), etcd.Spec.Size, "must >= 1 and <= 7"))
	}

	_, err := semver.Parse(etcd.Spec.Version)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("version"), etcd.Spec.Version, "must be a semver version"))
	}

	// external and autogenerate can't exist at the same time
	if etcd.Spec.Secure != nil && etcd.Spec.Secure.TLS != nil && etcd.Spec.Secure.TLS.ExternalCerts != nil && etcd.Spec.Secure.TLS.AutoTLSCert != nil {
		tls := etcd.Spec.Secure.TLS
		if tls.AutoTLSCert.AutoGenerateServerCert && tls.ExternalCerts.ServerSecret != "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("secure", "tls", "autoTLSCert", "autoGenerateServerCert"), tls.AutoTLSCert.AutoGenerateServerCert, "can't be specified at the same time as serverSecret"))
		}
		if tls.AutoTLSCert.AutoGeneratePeerCert && tls.ExternalCerts.PeerSecret != "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("secure", "tls", "autoTLSCert", "autoGeneratePeerCert"), tls.AutoTLSCert.AutoGeneratePeerCert, "can't be specified at the same time as peerSecret"))
		}
		if tls.AutoTLSCert.AutoGenerateClientCert && tls.ExternalCerts.ClientSecret != "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("secure", "tls", "autoTLSCert", "autoGenerateClientCert"), tls.AutoTLSCert.AutoGenerateClientCert, "can't be specified at the same time as clientSecret"))
		}
	}

	if len(etcd.Spec.Learners) > 1 {
		allErrs = append(allErrs, field.TooMany(fldPath.Child("learners"), len(etcd.Spec.Learners), 1))
	} else if len(etcd.Spec.Learners) == 1 {
		index, err := strconv.Atoi(etcd.Spec.Learners[0])
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("learners"), etcd.Spec.Learners, "must be an array of numeric strings"))
		} else if int32(index) >= etcd.Spec.Size {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("learners"), etcd.Spec.Learners, "must less than etcd size"))
		}
	}

	return allErrs
}

// ValidateUpdate validate if an etcd cluster updating is valid
func ValidateUpdate(oldE, newE *v1alpha1.EtcdCluster) field.ErrorList {
	allErrs := Validate(newE)
	if len(allErrs) > 0 {
		return allErrs
	}

	fldPath := field.NewPath("spec")

	oldV, _ := semver.Parse(oldE.Spec.Version)
	newV, _ := semver.Parse(newE.Spec.Version)
	if newV.LT(oldV) {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("version"), "version can't be downgrade"))
	}

	if !reflect.DeepEqual(oldE.Spec.Template.PersistentVolumeClaimSpec, newE.Spec.Template.PersistentVolumeClaimSpec) {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("template").Child("persistentVolumeClaimSpec"), "not allow updating now."))
	}

	if len(newE.Spec.Learners) > 0 {
		for _, member := range oldE.Status.Members {
			item := strings.Split(member.Name, "-")
			if item[len(item)-1] != newE.Spec.Learners[0] {
				continue
			}
			if member.Role == v1alpha1.EtcdMemberLeader || member.Role == v1alpha1.EtcdMemberFollower {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("learners"), newE.Spec.Learners, "this node has already been a vote member"))
			}
		}
	}

	return allErrs
}

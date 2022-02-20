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

package admission

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"

	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/server/mux"
	klog "k8s.io/klog/v2"

	etcdv1alpha1 "tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
	"tkestack.io/kstone-etcd-operator/pkg/validation"
)

// InstallHandler register handler for admission controller
func InstallHandler(mux *mux.PathRecorderMux) {
	mux.Handle("/mutating-etcd", &admissionHandler{
		delegate: &etcdMutatingHandler{},
	})
	mux.Handle("/validating-etcd", &admissionHandler{
		delegate: &etcdValidateHandler{},
	})
}

// Admit makes an admission decision based on the request attributes
type Admit interface {
	Admit(ar *v1.AdmissionReview) *v1.AdmissionResponse
}

type admissionHandler struct {
	delegate Admit
}

func (handler *admissionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	if r.Body == nil {
		return
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		msg := fmt.Sprintf("Request could not be read: %v", err)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	klog.V(3).Info(fmt.Sprintf("handling request: %s", body))

	requestAdmissionReview := &v1.AdmissionReview{}

	deserializer := codecs.UniversalDeserializer()
	_, gvk, err := deserializer.Decode(body, nil, requestAdmissionReview)
	if err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	responseAdmissionReview := &v1.AdmissionReview{}
	responseAdmissionReview.SetGroupVersionKind(*gvk)
	responseAdmissionReview.Response = handler.delegate.Admit(requestAdmissionReview)
	responseAdmissionReview.Response.UID = requestAdmissionReview.Request.UID

	respBytes, err := json.Marshal(responseAdmissionReview)
	if err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	klog.V(3).Info(fmt.Sprintf("sending response: %v", string(respBytes)))
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		klog.Error(err)
	}
}

type etcdMutatingHandler struct{}

// Admit mutate the etcd cluster's status and conditions
func (h *etcdMutatingHandler) Admit(ar *v1.AdmissionReview) *v1.AdmissionResponse {
	switch ar.Request.Operation {
	case v1.Create:
		var e etcdv1alpha1.EtcdCluster
		_, _, err := codecs.UniversalDeserializer().Decode(ar.Request.Object.Raw, nil, &e)
		if err != nil {
			return &v1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
		}
		patch, err := h.getCreatePatch(&e)
		if err != nil {
			return &v1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
		}
		return getPatchResponse(patch)
	case v1.Update:
		var newE, oldE etcdv1alpha1.EtcdCluster
		_, _, err := codecs.UniversalDeserializer().Decode(ar.Request.Object.Raw, nil, &newE)
		if err != nil {
			return &v1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
		}
		_, _, err = codecs.UniversalDeserializer().Decode(ar.Request.OldObject.Raw, nil, &oldE)
		if err != nil {
			return &v1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
		}
		patch, err := h.getUpdatePatch(&oldE, &newE)
		if err != nil {
			return &v1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
		}
		return getPatchResponse(patch)
	default:
	}
	return &v1.AdmissionResponse{Allowed: true}
}

// Patch define json patch type
type Patch struct {
	Op    string      `json:"op,inline"`
	Path  string      `json:"path,inline"`
	Value interface{} `json:"value"`
}

func (h *etcdMutatingHandler) getCreatePatch(e *etcdv1alpha1.EtcdCluster) ([]byte, error) {
	patches := make([]Patch, 0)

	out := e.DeepCopy()
	scheme.Default(out)
	if !reflect.DeepEqual(out.Spec, e.Spec) {
		patches = append(patches, Patch{
			Op:    "replace",
			Path:  "/spec",
			Value: out.Spec,
		})
	}

	patches = append(patches,
		Patch{
			Op:    "add",
			Path:  "/status",
			Value: struct{}{},
		},
		Patch{
			Op:    "add",
			Path:  "/status/phase",
			Value: etcdv1alpha1.ClusterCreating,
		})
	return json.Marshal(patches)
}

func (h *etcdMutatingHandler) getUpdatePatch(oldE, newE *etcdv1alpha1.EtcdCluster) ([]byte, error) {
	patches := make([]Patch, 0)
	out := newE.DeepCopy()
	scheme.Default(out)
	if !reflect.DeepEqual(out.Spec, newE.Spec) {
		patches = append(patches, Patch{
			Op:    "replace",
			Path:  "/spec",
			Value: out.Spec,
		})
	}
	if len(patches) == 0 {
		return nil, nil
	}
	return json.Marshal(patches)
}

type etcdValidateHandler struct{}

// Admit validate the etcdculster's specification
func (v *etcdValidateHandler) Admit(ar *v1.AdmissionReview) *v1.AdmissionResponse {
	switch ar.Request.Operation {
	case v1.Create:
		var e etcdv1alpha1.EtcdCluster
		_, _, err := codecs.UniversalDeserializer().Decode(ar.Request.Object.Raw, nil, &e)
		if err != nil {
			return &v1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
		}
		if errs := validation.Validate(&e); len(errs) > 0 {
			return &v1.AdmissionResponse{Result: &metav1.Status{Message: errs.ToAggregate().Error()}}
		}
	case v1.Update:
		var newE, oldE etcdv1alpha1.EtcdCluster
		_, _, err := codecs.UniversalDeserializer().Decode(ar.Request.Object.Raw, nil, &newE)
		if err != nil {
			return &v1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
		}
		_, _, err = codecs.UniversalDeserializer().Decode(ar.Request.OldObject.Raw, nil, &oldE)
		if err != nil {
			return &v1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
		}
		if errs := validation.ValidateUpdate(&oldE, &newE); len(errs) > 0 {
			return &v1.AdmissionResponse{Result: &metav1.Status{Message: errs.ToAggregate().Error()}}
		}
	default:
	}
	return &v1.AdmissionResponse{Allowed: true}
}

func getPatchResponse(patch []byte) *v1.AdmissionResponse {
	rsp := &v1.AdmissionResponse{Allowed: true}
	if len(patch) > 0 {
		rsp.Patch = patch
		pt := v1.PatchTypeJSONPatch
		rsp.PatchType = &pt
	}
	return rsp
}

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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"

	"tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
)

func TestNewTApp(t *testing.T) {
	vmode := corev1.PersistentVolumeFilesystem
	sc := "cbs"

	type args struct {
		e *v1alpha1.EtcdCluster
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "http",
			args: args{
				e: &v1alpha1.EtcdCluster{
					ObjectMeta: v1.ObjectMeta{
						Namespace: "etcd-demo",
						Name:      "example-etcd",
					},
					Spec: v1alpha1.EtcdClusterSpec{
						Size:       3,
						Version:    "3.4.13",
						ClientPort: pointer.Int32(2379),
					},
				},
			},
		},
		{
			name: "https",
			args: args{
				e: &v1alpha1.EtcdCluster{
					ObjectMeta: v1.ObjectMeta{
						Namespace: "etcd-demo",
						Name:      "example-etcd",
					},
					Spec: v1alpha1.EtcdClusterSpec{
						Size:       3,
						Version:    "3.4.13",
						ClientPort: pointer.Int32(2379),
						Secure: &v1alpha1.SecureConfig{
							TLS: &v1alpha1.TLSConfig{
								AutoTLSCert: &v1alpha1.AutoGenerateCertConfig{
									AutoGenerateServerCert: true,
									AutoGeneratePeerCert:   true,
									AutoGenerateClientCert: true,
								},
							},
						},
						Template: v1alpha1.EtcdTemplateSpec{
							Labels: map[string]string{
								"etcd-cluster": "example",
							},
							ExtraArgs: []string{"client-cert-auth=true"},
							PersistentVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceStorage: resource.MustParse("100Gi"),
									},
								},
								StorageClassName: &sc,
								VolumeMode:       &vmode,
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU: resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSeedTApp(tt.args.e)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSeedTApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = json.Marshal(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal failed: %v", err)
			}
		})
	}
}

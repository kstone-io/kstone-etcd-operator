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
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	tAppV1 "tkestack.io/tapp/pkg/apis/tappcontroller/v1"

	"tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
	"tkestack.io/kstone-etcd-operator/pkg/controller"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/constants"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/helpers"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/secure"
	"tkestack.io/kstone-etcd-operator/pkg/utils/flag"
)

// NewSeedTApp create a new TApp from an EtcdCluster template
func NewSeedTApp(e *v1alpha1.EtcdCluster) (*tAppV1.TApp, error) {
	app := &tAppV1.TApp{
		ObjectMeta: v1.ObjectMeta{
			Namespace: e.Namespace,
			Name:      helpers.NameForTApp(e),
		},
		Spec: tAppV1.TAppSpec{
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					constants.LabelEtcdClusterNameSelector: e.Name,
				},
			},
			UpdateStrategy: tAppV1.TAppUpdateStrategy{
				MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				ForceUpdate: tAppV1.ForceUpdateStrategy{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			AutoDeleteUnusedTemplate: true,
			Replicas:                 1,
			ServiceName:              helpers.NameForHeadlessService(e),
		},
	}

	for _, f := range []func(*v1alpha1.EtcdCluster, *tAppV1.TApp){
		ApplyEtcdDefaultTemplate,
		ApplyEtcdTLSOptions,
		ApplyEtcdPersistentVolume,
		ApplyEtcdNodeInitArgs,
		ApplyEtcdExtraArgs,
	} {
		f(e, app)
	}

	controller.SetEtcdOwnerReference(&app.ObjectMeta, e)

	return app, nil
}

// ApplyEtcdDefaultTemplate set default TApp template with the EtcdCluster template
func ApplyEtcdDefaultTemplate(e *v1alpha1.EtcdCluster, app *tAppV1.TApp) {
	app.Spec.Template = DefaultTAppTemplateForEtcd(e)
	app.Spec.ForceDeletePod = true
	for i := 0; i < int(app.Spec.Replicas); i++ {
		if app.Spec.TemplatePool == nil {
			app.Spec.TemplatePool = make(map[string]corev1.PodTemplateSpec)
		}
		tpl := DefaultTAppTemplateForEtcd(e)
		if len(e.Spec.Learners) != 0 && e.Spec.Learners[0] == fmt.Sprint(i) {
			tpl.Labels[constants.LabelEtcdVotingMemberSelector] = "false"
		}
		app.Spec.TemplatePool[fmt.Sprint(i)] = tpl
		if app.Spec.Templates == nil {
			app.Spec.Templates = make(map[string]string)
		}
		app.Spec.Templates[fmt.Sprint(i)] = fmt.Sprint(i)
	}
}

// DefaultTAppTemplateForEtcd create TApp template from the EtcdCluster template
func DefaultTAppTemplateForEtcd(e *v1alpha1.EtcdCluster) corev1.PodTemplateSpec {
	tpl := corev1.PodTemplateSpec{
		ObjectMeta: v1.ObjectMeta{
			Name:        helpers.NameForTApp(e),
			Labels:      e.Spec.Template.Labels,
			Annotations: e.Spec.Template.Annotations,
		},
		Spec: corev1.PodSpec{
			Affinity:                  e.Spec.Template.Affinity,
			Tolerations:               e.Spec.Template.Tolerations,
			TopologySpreadConstraints: e.Spec.Template.TopologySpreadConstraints,
			InitContainers: []corev1.Container{
				NewEtcdInitContainer(e),
			},
			Containers: []corev1.Container{
				{
					Name:      "etcd",
					Image:     helpers.ImageForEtcd(e),
					Command:   []string{"etcd"},
					Env:       e.Spec.Template.Env,
					Resources: e.Spec.Template.Resources,
					LivenessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(constants.EtcdDefaultPeerPort),
							},
						},
						InitialDelaySeconds: 5,
						TimeoutSeconds:      1,
						PeriodSeconds:       10,
						SuccessThreshold:    1,
						FailureThreshold:    5,
					},
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: func() intstr.IntOrString {
									if e.Spec.ClientPort != nil {
										return intstr.FromInt(int(*e.Spec.ClientPort))
									}
									return intstr.FromInt(constants.EtcdDefaultClientPort)
								}(),
							},
						},
						InitialDelaySeconds: 10,
						TimeoutSeconds:      1,
						PeriodSeconds:       10,
						SuccessThreshold:    1,
						FailureThreshold:    5,
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          constants.EtcdPeerPortName,
							ContainerPort: constants.EtcdDefaultPeerPort,
						},
						{
							Name: constants.EtcdClientPortName,
							ContainerPort: func() int32 {
								if e.Spec.ClientPort != nil {
									return *e.Spec.ClientPort
								}
								return constants.EtcdDefaultClientPort
							}(),
						},
					},
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
		},
	}
	if tpl.Labels == nil {
		tpl.Labels = make(map[string]string)
	}
	tpl.Labels[constants.LabelEtcdClusterNameSelector] = e.Name
	tpl.Labels[constants.LabelEtcdVotingMemberSelector] = "true"

	return tpl
}

// ApplyEtcdNodeInitArgs set the TApp container args with the EtcdCluster template
func ApplyEtcdNodeInitArgs(e *v1alpha1.EtcdCluster, app *tAppV1.TApp) {
	// append init args
	for i := 0; i < int(app.Spec.Replicas); i++ {
		args := make(map[string]string)
		args["name"] = app.Name + "-" + fmt.Sprint(i)
		if helpers.IsClientSecure(e) {
			args["trusted-ca-file"] = path.Join(constants.EtcdServerTLSMountPath, secure.EtcdCACertName)
			args["cert-file"] = path.Join(constants.EtcdServerTLSMountPath, secure.EtcdServerCertName)
			args["key-file"] = path.Join(constants.EtcdServerTLSMountPath, secure.EtcdServerKeyName)
		}
		args["listen-client-urls"] = fmt.Sprintf("%s://0.0.0.0:%d", helpers.SchemeForClient(e), func() int32 {
			if e.Spec.ClientPort != nil {
				return *e.Spec.ClientPort
			}
			return constants.EtcdDefaultClientPort
		}())
		args["advertise-client-urls"] = strings.Join(helpers.AdvertiseClientURLs(e), ",")

		if i == 0 {
			args["initial-cluster-state"] = "new"
			args["initial-cluster-token"] = e.Namespace + "-" + e.Name
		} else {
			args["initial-cluster-state"] = "existing"
		}

		args["initial-advertise-peer-urls"] = fmt.Sprintf("%s://%s:%d", helpers.SchemeForPeer(e), fmt.Sprintf("%s-%d.%s.%s.svc.%s", app.Name, i, helpers.NameForHeadlessService(e), e.Namespace, helpers.ClusterDomain(e)), constants.EtcdDefaultPeerPort)

		for j := 0; j <= i; j++ {
			if j > 0 {
				args["initial-cluster"] = args["initial-cluster"] + ","
			}
			args["initial-cluster"] += fmt.Sprintf("%s=%s://%s:%d", app.Name+"-"+fmt.Sprint(j), helpers.SchemeForPeer(e), fmt.Sprintf("%s-%d.%s.%s.svc.%s", app.Name, j, helpers.NameForHeadlessService(e), e.Namespace, helpers.ClusterDomain(e)), constants.EtcdDefaultPeerPort)
		}

		if helpers.IsPeerSecure(e) {
			args["peer-trusted-ca-file"] = path.Join(constants.EtcdPeerTLSMountPath, secure.EtcdCACertName)
			args["peer-cert-file"] = path.Join(constants.EtcdPeerTLSMountPath, secure.EtcdPeerCertName)
			args["peer-key-file"] = path.Join(constants.EtcdPeerTLSMountPath, secure.EtcdPeerKeyName)

		}
		args["listen-peer-urls"] = fmt.Sprintf("%s://0.0.0.0:%d", helpers.SchemeForPeer(e), constants.EtcdDefaultPeerPort)

		// pv args
		if e.Spec.Template.PersistentVolumeClaimSpec != nil {
			args["data-dir"] = constants.EtcdDataVolumeMountPath
		}

		// auth args
		// - --peer-client-cert-auth=true
		// - --client-cert-auth=true
		app.Spec.TemplatePool[fmt.Sprint(i)].Spec.Containers[0].Args = flag.ToStringSlice(args, "--")
	}
}

// ApplyEtcdExtraArgs  set the etcd extra args for TApp
func ApplyEtcdExtraArgs(e *v1alpha1.EtcdCluster, app *tAppV1.TApp) {
	for index, tpl := range app.Spec.TemplatePool {
		newTpl := tpl
		newTpl.Spec.Containers[0].Args = flag.Merge(tpl.Spec.Containers[0].Args, e.Spec.Template.ExtraArgs, "--", "--")
		app.Spec.TemplatePool[index] = newTpl
	}
}

// ApplyEtcdTLSOptions set the etcd tls option args for TApp
func ApplyEtcdTLSOptions(e *v1alpha1.EtcdCluster, app *tAppV1.TApp) {
	if e.Spec.Secure == nil || e.Spec.Secure.TLS == nil {
		// TODO: remove volume
		return
	}

	tlsConfig := e.Spec.Secure.TLS

	// append tls volume and mount
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	// using external cert
	if tlsConfig.ExternalCerts != nil && tlsConfig.ExternalCerts.ServerSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: constants.EtcdServerTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: tlsConfig.ExternalCerts.ServerSecret,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      constants.EtcdServerTLSVolumeName,
			MountPath: constants.EtcdServerTLSMountPath,
		})
	}
	if tlsConfig.ExternalCerts != nil && tlsConfig.ExternalCerts.PeerSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: constants.EtcdPeerTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: tlsConfig.ExternalCerts.PeerSecret,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      constants.EtcdPeerTLSVolumeName,
			MountPath: constants.EtcdPeerTLSMountPath,
		})
	}
	if tlsConfig.ExternalCerts != nil && tlsConfig.ExternalCerts.ClientSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: constants.EtcdClientTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: tlsConfig.ExternalCerts.ClientSecret,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      constants.EtcdClientTLSVolumeName,
			MountPath: constants.EtcdClientTLSMountPath,
		})
	}

	// using auto generate cert
	if tlsConfig.AutoTLSCert != nil && tlsConfig.AutoTLSCert.AutoGenerateServerCert {
		volumes = append(volumes, corev1.Volume{
			Name: constants.EtcdServerTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secure.NameForEtcdCertSecret(e.Name, secure.EtcdServerCert),
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      constants.EtcdServerTLSVolumeName,
			MountPath: constants.EtcdServerTLSMountPath,
		})
	}
	if tlsConfig.AutoTLSCert != nil && tlsConfig.AutoTLSCert.AutoGeneratePeerCert {
		volumes = append(volumes, corev1.Volume{
			Name: constants.EtcdPeerTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secure.NameForEtcdCertSecret(e.Name, secure.EtcdPeerCert),
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      constants.EtcdPeerTLSVolumeName,
			MountPath: constants.EtcdPeerTLSMountPath,
		})
	}
	if tlsConfig.AutoTLSCert != nil && tlsConfig.AutoTLSCert.AutoGenerateClientCert {
		volumes = append(volumes, corev1.Volume{
			Name: constants.EtcdClientTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secure.NameForEtcdCertSecret(e.Name, secure.EtcdClientCert),
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      constants.EtcdClientTLSVolumeName,
			MountPath: constants.EtcdClientTLSMountPath,
		})
	}

	for k, tp := range app.Spec.TemplatePool {
		tp.Spec.Volumes = append(tp.Spec.Volumes, volumes...)
		app.Spec.TemplatePool[k] = tp
	}

	for i := range app.Spec.TemplatePool {
		app.Spec.TemplatePool[i].Spec.Containers[0].VolumeMounts = append(app.Spec.TemplatePool[i].Spec.Containers[0].VolumeMounts, volumeMounts...)
	}
}

// ApplyEtcdPersistentVolume set the TApp persistent volume option with EtcdCluster's persistent volume claim spec
func ApplyEtcdPersistentVolume(e *v1alpha1.EtcdCluster, app *tAppV1.TApp) {
	if e.Spec.Template.PersistentVolumeClaimSpec != nil {
		app.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: v1.ObjectMeta{
					Name: constants.EtcdDataVolumeName,
				},
				Spec: *e.Spec.Template.PersistentVolumeClaimSpec,
			},
		}
		for i := range app.Spec.TemplatePool {
			app.Spec.TemplatePool[i].Spec.Containers[0].VolumeMounts = append(app.Spec.TemplatePool[i].Spec.Containers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      constants.EtcdDataVolumeName,
					MountPath: constants.EtcdDataVolumeMountPath,
				})
		}
	}
}

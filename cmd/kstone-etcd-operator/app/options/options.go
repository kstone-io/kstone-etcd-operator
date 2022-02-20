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

package options

import (
	"fmt"
	"net"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	apiserver "k8s.io/apiserver/pkg/server"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	cliflag "k8s.io/component-base/cli/flag"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/metrics"
	cmcfg "k8s.io/controller-manager/config"
	controlleroptions "k8s.io/controller-manager/options"

	"tkestack.io/kstone-etcd-operator/cmd/kstone-etcd-operator/app/config"
)

const (
	// KStoneEtcdOperatorUserAgent is the userAgent name when starting kstone-etcd-operator.
	KStoneEtcdOperatorUserAgent = "kstone-etcd-operator"
	ServerName                  = "kstone-etcd-operator"
)

const (
	defaultConcurrentSyncs = 10
	defaultPort            = 443
)

// Options is the main context object for the TKE controller manager.
type Options struct {
	SecureServing         *apiserveroptions.SecureServingOptions
	Generic               *controlleroptions.GenericControllerManagerConfigurationOptions
	Metrics               *metrics.Options
	Logs                  *logs.Options
	Master                string
	Kubeconfig            string
	EtcdClusterController *EtcdClusterOptions
}

// NewKStoneEtcdOperatorOptions creates a new Options with a default config.
func NewKStoneEtcdOperatorOptions() *Options {
	componentConfig := NewDefaultGenericConfig()
	return &Options{
		SecureServing:         NewDefaultSecureServingOptions(ServerName, defaultPort),
		Generic:               controlleroptions.NewGenericControllerManagerConfigurationOptions(&componentConfig),
		Metrics:               metrics.NewOptions(),
		Logs:                  logs.NewOptions(),
		EtcdClusterController: NewEtcdClusterControllerOptions(),
	}
}

func NewDefaultSecureServingOptions(serverName string, defaultPort int) *apiserveroptions.SecureServingOptions {
	o := apiserveroptions.NewSecureServingOptions()
	o.ServerCert.PairName = serverName
	o.ServerCert.CertDirectory = ""
	o.BindPort = defaultPort
	return o
}

// NewDefaultGenericConfig returns kube-controller manager configuration object.
func NewDefaultGenericConfig() cmcfg.GenericControllerManagerConfiguration {
	return cmcfg.GenericControllerManagerConfiguration{
		Port:                    443,
		MinResyncPeriod:         metav1.Duration{Duration: 12 * time.Hour},
		ControllerStartInterval: metav1.Duration{Duration: 0 * time.Second},
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:   true,
			LeaseDuration: metav1.Duration{Duration: 15 * time.Second},
			RenewDeadline: metav1.Duration{Duration: 10 * time.Second},
			RetryPeriod:   metav1.Duration{Duration: 2 * time.Second},
		},
		Controllers: []string{"*"},
	}
}

// Flags returns flags for a specific app by section name
func (o *Options) Flags(allControllers []string, disabledByDefaultControllers []string) cliflag.NamedFlagSets {
	fss := cliflag.NamedFlagSets{}
	o.Generic.AddFlags(&fss, allControllers, disabledByDefaultControllers)
	o.SecureServing.AddFlags(fss.FlagSet("secure serving"))

	o.EtcdClusterController.AddFlags(fss.FlagSet("etcdcluster controller"))

	o.Metrics.AddFlags(fss.FlagSet("metrics"))
	o.Logs.AddFlags(fss.FlagSet("logs"))

	fs := fss.FlagSet("misc")
	fs.StringVar(&o.Master, "master", o.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig).")
	fs.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	return fss
}

func (o *Options) ApplyTo(c *config.Config) error {
	if err := o.Generic.ApplyTo(&c.Generic); err != nil {
		return err
	}
	if err := o.SecureServing.ApplyTo(&c.SecureServing); err != nil {
		return err
	}
	if err := o.EtcdClusterController.ApplyTo(&c.EtcdClusterController); err != nil {
		return err
	}
	return nil
}

func (o *Options) Validate(allControllers []string, disabledByDefaultControllers []string) error {
	var errs []error
	errs = append(errs, o.Generic.Validate(allControllers, disabledByDefaultControllers)...)
	errs = append(errs, o.Metrics.Validate()...)
	errs = append(errs, o.Logs.Validate()...)
	errs = append(errs, o.EtcdClusterController.Validate()...)
	return utilerrors.NewAggregate(errs)
}

func (o Options) Config(allControllers []string, disabledByDefaultControllers []string) (*config.Config, error) {
	if err := o.Validate(allControllers, disabledByDefaultControllers); err != nil {
		return nil, err
	}
	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	kubeconfig, err := clientcmd.BuildConfigFromFlags(o.Master, o.Kubeconfig)
	if err != nil {
		return nil, err
	}
	kubeconfig.DisableCompression = true
	kubeconfig.ContentConfig.AcceptContentTypes = o.Generic.ClientConnection.AcceptContentTypes
	kubeconfig.ContentConfig.ContentType = o.Generic.ClientConnection.ContentType
	kubeconfig.QPS = o.Generic.ClientConnection.QPS
	kubeconfig.Burst = int(o.Generic.ClientConnection.Burst)

	client, err := clientset.NewForConfig(restclient.AddUserAgent(kubeconfig, KStoneEtcdOperatorUserAgent))
	if err != nil {
		return nil, err
	}

	eventRecorder := createRecorder(client, KStoneEtcdOperatorUserAgent)

	cfg := &config.Config{
		ServerName: ServerName,
		Authentication: apiserver.AuthenticationInfo{
			Authenticator: anonymous.NewAuthenticator(),
		},
		Authorization: apiserver.AuthorizationInfo{
			Authorizer: authorizerfactory.NewAlwaysAllowAuthorizer(),
		},
		ClientConfig:  kubeconfig,
		EventRecorder: eventRecorder,
	}
	if err := o.ApplyTo(cfg); err != nil {
		return nil, err
	}

	o.Metrics.Apply()
	o.Logs.Apply()

	return cfg, nil
}

func createRecorder(kubeClient clientset.Interface, userAgent string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: userAgent})
}

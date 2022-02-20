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

package app

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	kubeInformer "k8s.io/client-go/informers"
	"k8s.io/client-go/restmapper"
	genericcontrollermanager "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/pkg/clientbuilder"
	tAppClientSet "tkestack.io/tapp/pkg/client/clientset/versioned"
	tAppInformer "tkestack.io/tapp/pkg/client/informers/externalversions"

	"tkestack.io/kstone-etcd-operator/cmd/kstone-etcd-operator/app/config"
	etcdClientSet "tkestack.io/kstone-etcd-operator/pkg/client/clientset/versioned"
	etcdInformer "tkestack.io/kstone-etcd-operator/pkg/client/informers/externalversions"
)

// InitFunc is used to launch a particular controller.  It may run additional "should I activate checks".
// Any error returned will cause the controller process to `Fatal`
// The bool indicates whether the controller was enabled.
type InitFunc func(ctx context.Context, controllerCtx ControllerContext) (debuggingHandler http.Handler, enabled bool, err error)

// ControllerContext represents the context of controller.
type ControllerContext struct {
	// KubeClientBuilder will provide a client for this controller to use
	KubeClientBuilder clientbuilder.ControllerClientBuilder

	// KubeInformerFactory gives access to etcd informers for the controller.
	KubeInformerFactory kubeInformer.SharedInformerFactory
	// EtcdInformerFactory gives access to etcd informers for the controller.
	EtcdInformerFactory etcdInformer.SharedInformerFactory
	// TAppInformerFactory gives access to tapp informers for the controller.
	TAppInformerFactory tAppInformer.SharedInformerFactory

	// Config provides access to init options for a given controller
	Config config.Config

	// DeferredDiscoveryRESTMapper is a RESTMapper that will defer
	// initialization of the RESTMapper until the first mapping is
	// requested.
	RESTMapper *restmapper.DeferredDiscoveryRESTMapper

	// AvailableResources is a map listing currently available resources
	AvailableResources map[schema.GroupVersionResource]bool

	// InformersStarted is closed after all of the controllers have been initialized and are running.  After this point it is safe,
	// for an individual controller to start the shared informers. Before it is closed, they should not.
	InformersStarted chan struct{}

	// ResyncPeriod generates a duration each time it is invoked; this is so that
	// multiple controllers don't get into lock-step and all hammer the apiserver
	// with list requests simultaneously.
	ResyncPeriod            func() time.Duration
	ControllerStartInterval metav1.Duration

	TAppClient tAppClientSet.Interface
	EtcdClient etcdClientSet.Interface
}

// IsControllerEnabled returns whether the controller has been enabled
func (c ControllerContext) IsControllerEnabled(name string) bool {
	return genericcontrollermanager.IsControllerEnabled(name, ControllersDisabledByDefault, c.Config.Generic.Controllers)
}

// ResyncPeriod returns a function which generates a duration each time it is
// invoked; this is so that multiple controllers don't get into lock-step and all
// hammer the apiserver with list requests simultaneously.
func ResyncPeriod(c *config.Config) func() time.Duration {
	return func() time.Duration {
		factor := rand.Float64() + 1
		return time.Duration(float64(c.Generic.MinResyncPeriod.Nanoseconds()) * factor)
	}
}

func GetAvailableResources(clientBuilder clientbuilder.ControllerClientBuilder) (map[schema.GroupVersionResource]bool, error) {
	client := clientBuilder.ClientOrDie("controller-discovery")
	discoveryClient := client.Discovery()
	_, resourceMap, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get all supported resources from server: %v", err))
	}
	if len(resourceMap) == 0 {
		return nil, fmt.Errorf("unable to get any supported resources from server")
	}

	allResources := map[schema.GroupVersionResource]bool{}
	for _, apiResourceList := range resourceMap {
		version, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			return nil, err
		}
		for _, apiResource := range apiResourceList.APIResources {
			allResources[version.WithResource(apiResource.Name)] = true
		}
	}

	return allResources, nil
}

// CreateControllerContext creates a context struct containing references to resources needed by the controllers.
func CreateControllerContext(ctx context.Context, cfg *config.Config, kubeClientBuilder clientbuilder.ControllerClientBuilder) (ControllerContext, error) {

	versionedClient := kubeClientBuilder.ClientOrDie("shared-informers")
	sharedInformers := kubeInformer.NewSharedInformerFactory(versionedClient, ResyncPeriod(cfg)())

	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	if err := genericcontrollermanager.WaitForAPIServer(versionedClient, 10*time.Second); err != nil {
		return ControllerContext{}, fmt.Errorf("failed to wait for apiserver being healthy: %v", err)
	}

	// Use a discovery client capable of being refreshed.
	discoveryClient := kubeClientBuilder.DiscoveryClientOrDie("controller-discovery")
	cachedClient := cacheddiscovery.NewMemCacheClient(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)
	go wait.Until(func() {
		restMapper.Reset()
	}, 30*time.Second, ctx.Done())

	availableResources, err := GetAvailableResources(kubeClientBuilder)
	if err != nil {
		return ControllerContext{}, err
	}

	// create etcd informers
	etcdClient := etcdClientSet.NewForConfigOrDie(kubeClientBuilder.ConfigOrDie("kstone-etcd-operator"))
	etcdInformers := etcdInformer.NewSharedInformerFactory(etcdClient, ResyncPeriod(cfg)())

	// create TApp informers
	tappClient := tAppClientSet.NewForConfigOrDie(kubeClientBuilder.ConfigOrDie("kstone-etcd-operator"))
	tappInformers := tAppInformer.NewSharedInformerFactory(tappClient, ResyncPeriod(cfg)())

	controllerContext := ControllerContext{
		KubeClientBuilder:       kubeClientBuilder,
		KubeInformerFactory:     sharedInformers,
		EtcdInformerFactory:     etcdInformers,
		TAppInformerFactory:     tappInformers,
		Config:                  *cfg,
		RESTMapper:              restMapper,
		AvailableResources:      availableResources,
		InformersStarted:        make(chan struct{}),
		ResyncPeriod:            ResyncPeriod(cfg),
		ControllerStartInterval: cfg.Generic.ControllerStartInterval,
		TAppClient:              tappClient,
		EtcdClient:              etcdClient,
	}

	return controllerContext, nil
}

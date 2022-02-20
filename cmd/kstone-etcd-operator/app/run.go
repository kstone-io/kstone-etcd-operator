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
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apiserver/pkg/server/routes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/component-base/logs"
	genericcontrollermanager "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/pkg/clientbuilder"
	klog "k8s.io/klog/v2"

	"tkestack.io/kstone-etcd-operator/cmd/kstone-etcd-operator/app/config"
	"tkestack.io/kstone-etcd-operator/pkg/admission"
)

// Run runs the specified platform controller manager. This should never exit.
func Run(ctx context.Context, cfg *config.Config) error {
	klog.Info("Starting kstone etcd operator")

	// Setup any health checks we will want to use.
	var checks []healthz.HealthChecker
	var electionChecker *leaderelection.HealthzAdaptor
	if cfg.Generic.LeaderElection.LeaderElect {
		electionChecker = leaderelection.NewLeaderHealthzAdaptor(time.Second * 20)
		checks = append(checks, electionChecker)
	}

	// Start the controller manager HTTP server
	// serverMux is the handler for these controller *after* authn/authz filters have been applied
	serverMux := genericcontrollermanager.NewBaseHandler(&cfg.Generic.Debugging, checks...)
	routes.DebugFlags{}.Install(serverMux, "v", routes.StringFlagPutHandler(logs.GlogSetter))
	// handle admission control
	admission.InstallHandler(serverMux)
	handler := genericcontrollermanager.BuildHandlerChain(serverMux, &cfg.Authorization, &cfg.Authentication)
	if _, err := cfg.SecureServing.Serve(handler, 0, ctx.Done()); err != nil {
		return err
	}

	run := func(ctx context.Context) {
		rootClientBuilder := clientbuilder.SimpleControllerClientBuilder{
			ClientConfig: cfg.ClientConfig,
		}

		controllerContext, err := CreateControllerContext(ctx, cfg, rootClientBuilder)
		if err != nil {
			klog.Fatalf("error building controller context: %v", err)
		}

		if err := StartControllers(ctx, controllerContext, NewControllerInitializers(), serverMux); err != nil {
			klog.Fatalf("error starting controllers: %v", err)
		}

		controllerContext.KubeInformerFactory.Start(ctx.Done())
		controllerContext.TAppInformerFactory.Start(ctx.Done())
		controllerContext.EtcdInformerFactory.Start(ctx.Done())
		close(controllerContext.InformersStarted)

		select {}
	}

	if !cfg.Generic.LeaderElection.LeaderElect {
		run(ctx)
		panic("unreachable")
	}

	id, err := os.Hostname()
	if err != nil {
		return err
	}

	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id = id + "_" + string(uuid.NewUUID())
	rl, err := resourcelock.NewFromKubeconfig(resourcelock.EndpointsResourceLock,
		cfg.Generic.LeaderElection.ResourceNamespace,
		cfg.ServerName,
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: cfg.EventRecorder,
		},
		cfg.ClientConfig,
		cfg.Generic.LeaderElection.RenewDeadline.Duration)
	if err != nil {
		klog.Fatalf("error creating lock: %v", err)
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: cfg.Generic.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: cfg.Generic.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   cfg.Generic.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("leaderelection lost")
			},
		},
		WatchDog: electionChecker,
		Name:     cfg.ServerName,
	})
	panic("unreachable")
}

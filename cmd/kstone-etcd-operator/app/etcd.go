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
	"net/http"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"tkestack.io/kstone-etcd-operator/pkg/apis/etcd"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster"
)

func startEtcdClusterController(ctx context.Context, controllerCtx ControllerContext) (debuggingHandler http.Handler, enabled bool, err error) {
	if !controllerCtx.AvailableResources[schema.GroupVersionResource{Group: etcd.GroupName, Version: "v1alpha1", Resource: "etcdclusters"}] {
		return nil, false, nil
	}

	ctrl := etcdcluster.NewEtcdClusterController(
		controllerCtx.Config.EtcdClusterController.ReconcileInterval,
		controllerCtx.Config.EtcdClusterController.StatusReconcileInterval,
		controllerCtx.EtcdClient,
		controllerCtx.TAppClient,
		controllerCtx.KubeClientBuilder.ClientOrDie("etcdcluster-controller"),
		controllerCtx.EtcdInformerFactory.Etcd().V1alpha1().EtcdClusters(),
		controllerCtx.TAppInformerFactory.Tappcontroller().V1().TApps(),
		controllerCtx.KubeInformerFactory.Core().V1().Pods(),
		controllerCtx.KubeInformerFactory.Core().V1().Services(),
		controllerCtx.KubeInformerFactory.Core().V1().Secrets(),
		controllerCtx.KubeInformerFactory.Core().V1().ConfigMaps(),
	)

	go ctrl.Run(ctx, controllerCtx.Config.EtcdClusterController.ConcurrentEtcdClusterSyncs)

	return nil, true, nil
}

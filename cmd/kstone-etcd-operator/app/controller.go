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
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/mux"
	klog "k8s.io/klog/v2"
)

const (
	// ControllerStartJitter used when starting controller managers.
	ControllerStartJitter = 1.0
)

// ControllersDisabledByDefault configured all controllers that are turned off
// by default.
var ControllersDisabledByDefault = sets.NewString()

// KnownControllers returns the known controllers.
func KnownControllers() []string {
	ret := sets.StringKeySet(NewControllerInitializers())
	return ret.List()
}

// NewControllerInitializers is a public map of named controller groups (you can start more than one in an init func)
// paired to their InitFunc.  This allows for structured downstream composition and subdivision.
func NewControllerInitializers() map[string]InitFunc {
	controllers := map[string]InitFunc{}

	controllers["etcdcluster"] = startEtcdClusterController
	return controllers
}

// StartControllers to start the controller.
func StartControllers(ctx context.Context, controllerCtx ControllerContext, controllers map[string]InitFunc, unsecuredMux *mux.PathRecorderMux) error {
	for controllerName, initFn := range controllers {
		if !controllerCtx.IsControllerEnabled(controllerName) {
			klog.Warningf("%q is disabled", controllerName)
			continue
		}

		time.Sleep(wait.Jitter(controllerCtx.ControllerStartInterval.Duration, ControllerStartJitter))

		klog.V(1).Infof("Starting %q", controllerName)
		debugHandler, started, err := initFn(ctx, controllerCtx)
		if err != nil {
			klog.Errorf("Error starting %q", controllerName)
			return err
		}
		if !started {
			klog.Warningf("Skipping %q", controllerName)
			continue
		}
		if debugHandler != nil && unsecuredMux != nil {
			basePath := "/debug/controllers/" + controllerName
			unsecuredMux.UnlistedHandle(basePath, http.StripPrefix(basePath, debugHandler))
			unsecuredMux.UnlistedHandlePrefix(basePath+"/", http.StripPrefix(basePath, debugHandler))
		}
		klog.Infof("Started %q", controllerName)
	}

	return nil
}

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
	"time"

	"github.com/spf13/pflag"

	etcdclusterconfig "tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/config"
)

// EtcdClusterOptions holds the EtcdClusterController options
type EtcdClusterOptions struct {
	*etcdclusterconfig.EtcdClusterControllerConfiguration
}

// NewEtcdClusterControllerOptions creates a new Options with a default config.
func NewEtcdClusterControllerOptions() *EtcdClusterOptions {
	return &EtcdClusterOptions{
		&etcdclusterconfig.EtcdClusterControllerConfiguration{
			ConcurrentEtcdClusterSyncs: defaultConcurrentSyncs,
			ReconcileInterval:          60 * time.Second,
			StatusReconcileInterval:    30 * time.Second,
		},
	}
}

// AddFlags adds flags related to EtcdClusterController for etcd cluster controller to the specified FlagSet.
func (o *EtcdClusterOptions) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}
	fs.IntVar(&o.ConcurrentEtcdClusterSyncs, "concurrent-etcdcluster-syncs", o.ConcurrentEtcdClusterSyncs, "The number of etcdcluster objects that are allowed to sync concurrently")
	fs.DurationVar(&o.ReconcileInterval, "etcdcluster-reconcile-interval", o.ReconcileInterval, "etcd cluster reconcile interval")
	fs.DurationVar(&o.StatusReconcileInterval, "etcdcluster-status-reconcile-interval", o.StatusReconcileInterval, "etcd cluster status reconcile interval")
}

// ApplyTo fills up EtcdClusterController config with options.
func (o *EtcdClusterOptions) ApplyTo(cfg *etcdclusterconfig.EtcdClusterControllerConfiguration) error {
	if o == nil {
		return nil
	}
	cfg.ConcurrentEtcdClusterSyncs = o.ConcurrentEtcdClusterSyncs
	cfg.ReconcileInterval = o.ReconcileInterval
	cfg.StatusReconcileInterval = o.StatusReconcileInterval
	return nil
}

// Validate checks validation of EtcdClusterOptions.
func (o *EtcdClusterOptions) Validate() []error {
	if o == nil {
		return nil
	}
	errs := []error{}
	return errs
}

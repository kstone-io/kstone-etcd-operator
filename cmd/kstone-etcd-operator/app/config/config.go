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

package config

import (
	apiserver "k8s.io/apiserver/pkg/server"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/controller-manager/config"

	etcdclusterconfig "tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/config"
)

const (
	// KStoneEtcdOperatorUserAgent is the userAgent name when starting kstone-etcd-operator.
	KStoneEtcdOperatorUserAgent = "kstone-etcd-operator"
)

type Config struct {
	SecureServing  *apiserver.SecureServingInfo
	Authentication apiserver.AuthenticationInfo
	Authorization  apiserver.AuthorizationInfo
	ServerName     string

	ClientConfig *restclient.Config
	// the event sink
	EventRecorder record.EventRecorder

	Generic               config.GenericControllerManagerConfiguration
	EtcdClusterController etcdclusterconfig.EtcdClusterControllerConfiguration
}

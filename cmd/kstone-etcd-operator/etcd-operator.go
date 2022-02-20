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

package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugin
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration

	"tkestack.io/kstone-etcd-operator/cmd/kstone-etcd-operator/app"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)

	command := app.NewKStoneEtcdOperatorCommand()

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

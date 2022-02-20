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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	apiserver "k8s.io/apiserver/pkg/server"
	restclient "k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/term"

	"tkestack.io/kstone-etcd-operator/cmd/kstone-etcd-operator/app/options"
	"tkestack.io/kstone-etcd-operator/pkg/version"
)

const commandDesc = `The kstone etcd operator is a daemon for managing etcd clusters`

// NewKStoneEtcdOperatorCommand creates a *cobra.Command object with default parameters
func NewKStoneEtcdOperatorCommand() *cobra.Command {
	o := options.NewKStoneEtcdOperatorOptions()

	cmd := &cobra.Command{
		Use:  "kstone-etcd-operator",
		Long: commandDesc,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			restclient.SetDefaultWarningHandler(restclient.NoWarnings{})
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			version.PrintAndExitIfRequested("kstone-etcd-operator")
			cliflag.PrintFlags(cmd.Flags())

			c, err := o.Config(KnownControllers(), ControllersDisabledByDefault.List())
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			ctx := apiserver.SetupSignalContext()
			if err := Run(ctx, c); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	fs := cmd.Flags()
	namedFlagSets := o.Flags(KnownControllers(), ControllersDisabledByDefault.List())
	version.AddFlags(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), cmd.Name())
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}
	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), namedFlagSets, cols)
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, cols)
	})

	return cmd
}

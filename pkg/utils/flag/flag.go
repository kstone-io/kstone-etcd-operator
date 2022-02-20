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

package flag

import (
	"fmt"
	"sort"
	"strings"
)

// Merge will merge the old flags string slice and new flags string slice, the new flags will override the old flags,
// the result will remove the trimPrefix if provided, add the addPrefix if provided, for example:
// Merge(["a=1"], ["--b=2"], "--", "--") will return ["--a=1", "--b=2"]
// Merge(["--a=1"], ["b=2"], "--", "") will return ["a=1", "b=2]
func Merge(old, new []string, trimPrefix, addPrefix string) []string {
	flagMap := ToMap(old, trimPrefix)
	extraFlagMap := ToMap(new, trimPrefix)
	for k, v := range extraFlagMap {
		flagMap[k] = v
	}
	return ToStringSlice(flagMap, addPrefix)
}

// ToMap converts the flags string slice to a map, the map key removes the trimPrefix prefix
func ToMap(flags []string, trimPrefix string) map[string]string {
	flagMap := make(map[string]string)
	for _, flag := range flags {
		if len(flag) == 0 {
			continue
		}
		part := strings.SplitN(flag, "=", 2)
		k := strings.TrimPrefix(strings.TrimSpace(part[0]), trimPrefix)
		if len(part) == 1 {
			flagMap[k] = "true"
		} else {
			flagMap[k] = strings.TrimSpace(part[1])
		}
	}
	return flagMap
}

// ToStringSlice convert flag map to flag string slice, adding prefix to the flag key if provided
func ToStringSlice(flagMap map[string]string, prefix string) []string {
	flagSlice := make([]string, 0)
	for k, v := range flagMap {
		if len(v) == 0 {
			flagSlice = append(flagSlice, fmt.Sprintf("%s%s", prefix, k))
		} else {
			flagSlice = append(flagSlice, fmt.Sprintf("%s%s=%s", prefix, k, v))
		}
	}
	sort.Strings(flagSlice)
	return flagSlice
}

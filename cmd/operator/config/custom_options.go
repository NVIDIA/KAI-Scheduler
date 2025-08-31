// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"regexp"
	"strings"
)

type ArrayFlags []string

func (i *ArrayFlags) String() string {
	return "Flag list"
}

func (i *ArrayFlags) Set(value string) error {
	compiledRegExp := regexp.MustCompile(",\\s+")
	valueRemovedWhiteSpaces := compiledRegExp.ReplaceAllString(value, ",")
	*i = strings.Split(valueRemovedWhiteSpaces, ",")
	return nil
}

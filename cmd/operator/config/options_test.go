// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"flag"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("options", Ordered, func() {
	Context("options", func() {
		It("Test full options", func() {
			flags := []string{
				"--namespace",
				"test",
				"--image-pull-secret",
				"runai-reg-creds",
				"--additional-image-pull-secrets",
				"pull_secret_1,pull_secret_2",
				"--cm-file-path=./test/params_all_set.json"}
			os.Args = append(os.Args, flags...)
			flagSet := flag.NewFlagSet("test1", flag.ExitOnError)
			opts, err := parse(flagSet, flags)
			Expect(err).To(BeNil())
			Expect(opts.Namespace).To(Equal("test"))
			Expect(opts.ImagePullSecretName).To(Equal("runai-reg-creds"))
			Expect(opts.CMConfigParams.Affinity).To(Not(BeNil()))
			Expect(len(opts.CMConfigParams.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms)).To(Equal(1))
			Expect(opts.CMConfigParams.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution == nil).To(BeTrue())
			Expect(opts.CMConfigParams.Tolerations).To(Not(BeNil()))
			Expect(len(opts.CMConfigParams.Tolerations)).To(Equal(2))
		})
		It("Test empty additional image pull secrets", func() {
			flags := []string{
				"--namespace",
				"test"}
			opts := &Options{}
			flagSet := flag.NewFlagSet("test2", flag.ExitOnError)
			opts.parseCommandLineArgs(flagSet, flags)
		})

		It("Test cm file doesn't exists flags parsed", func() {
			flags := []string{
				"--namespace",
				"test",
				"--image-pull-secret",
				"runai-reg-creds",
				"--additional-image-pull-secrets",
				"pull_secret_1,pull_secret_2"}
			flagSet := flag.NewFlagSet("test2", flag.ExitOnError)
			opts, err := parse(flagSet, flags)
			Expect(err).To(BeNil())
			Expect(opts.Namespace).To(Equal("test"))
			Expect(opts.ImagePullSecretName).To(Equal("runai-reg-creds"))
			Expect(opts.CMConfigParams.Affinity).To(BeNil())
			Expect(opts.CMConfigParams.Tolerations).To(BeNil())
		})
	})
})

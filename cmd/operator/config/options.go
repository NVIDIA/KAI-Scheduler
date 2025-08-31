// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	cmFile = "/etc/cm/params.json"
)

type CMConfigParams struct {
	Affinity    *v1.Affinity    `json:"affinity,omitempty"`
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
}

type Options struct {
	MetricsAddr                string
	EnableLeaderElection       bool
	ProbeAddr                  string
	Qps                        int
	Burst                      int
	Namespace                  string
	ImagePullSecretName        string
	AdditionalImagePullSecrets ArrayFlags
	ZapOptions                 zap.Options
	cMFilePath                 string
	CMConfigParams             CMConfigParams
}

func SetOptions() (*Options, error) {
	return parse(flag.CommandLine, os.Args[1:])
}

func parse(flagSet *flag.FlagSet, options []string) (*Options, error) {
	opts := &Options{}
	if err := opts.parseCommandLineArgs(flagSet, options); err != nil {
		return nil, err
	}
	if err := opts.loadConfigFromFile(); err != nil {
		return nil, err
	}
	return opts, nil
}

func (opts *Options) loadConfigFromFile() error {
	fileContent, err := os.ReadFile(opts.cMFilePath)
	if err != nil {
		log.Printf("configmap file not found, err: [%v]", err)
		return nil
	}
	err = json.Unmarshal(fileContent, &opts.CMConfigParams)
	if err != nil {
		return fmt.Errorf("failed to unmarshal configmap file content, err: [%v]", err)
	}
	return nil
}

func (opts *Options) parseCommandLineArgs(flagSet *flag.FlagSet, options []string) error {
	flagSet.StringVar(&opts.MetricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flagSet.StringVar(&opts.ProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flagSet.BoolVar(&opts.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flagSet.IntVar(&opts.Qps, "qps", 50, "Queries per second to the K8s API server")
	flagSet.IntVar(&opts.Burst, "burst", 300, "Burst to the K8s API server")

	flagSet.StringVar(&opts.Namespace, "namespace", "runai-engine", "The namespace to create the resources in")
	flagSet.StringVar(&opts.ImagePullSecretName, "image-pull-secret", "", "The name of the image pull secret to use")
	flagSet.Var(&opts.AdditionalImagePullSecrets, "additional-image-pull-secrets", "Additional image pull secrets names to use")
	flagSet.StringVar(&opts.cMFilePath, "cm-file-path", cmFile, "Path to the configmap file")
	opts.ZapOptions = zap.Options{
		Development: true,
	}
	opts.ZapOptions.BindFlags(flagSet)
	return flagSet.Parse(options)
}

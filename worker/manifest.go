package main

import (
	"io/ioutil"
	"os"
	"path"

	"gopkg.in/yaml.v2"
)

type Manifest struct {
	Arch         *string
	Artifacts    []string
	Environment  map[string]interface{}
	Image        string
	Packages     []string
	Repositories map[string]string
	Secrets      []string
	Shell        bool
	Sources      []string
	Tasks        []map[string]string
	Triggers     []map[string]interface{}
}

type ImageConfig struct {
	LoginCmd   string `yaml:"logincmd"`
	GitVariant string `yaml:"git_variant"`
	Homedir    string `yaml:"homedir"`
	Preamble   string `yaml:"preamble"`
}

func LoadImageConfig(image string) *ImageConfig {
	images, _ := config.Get("builds.sr.ht::worker", "images")
	iconf := &ImageConfig{
		LoginCmd:   "ssh",
		GitVariant: "git",
		Homedir:    "/home/build",
		Preamble:   `#!/usr/bin/env bash
. ~/.buildenv
set -xe
`,
	}
	f, err := os.Open(path.Join(images, image, "config.yml"))
	if err != nil {
		return iconf
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(b, iconf)
	if err != nil {
		panic(err)
	}
	return iconf
}

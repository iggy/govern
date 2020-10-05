// Copyright © 2020 Iggy <iggy@theiggy.com>
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice,
//    this list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its contributors
//    may be used to endorse or promote products derived from this software
//    without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package laws

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/iggy/govern/pkg/facts"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Package - package info
type Package struct {
	Name    string
	Version string `yaml:",omitempty"`
	// Installed bool   `yaml:",omitempty"` // whether the package should be installed or removed

	CommonFields
}

// UnmarshalYAML - This fills in default values if they aren't specified
func (p *Package) UnmarshalYAML(value *yaml.Node) error {
	pkg := &Package{}
	pkg.Present = true // present gets set to true because they wouldn't mention it otherwise
	var err error      // for use in the switch below

	log.Trace().Interface("Node", value).Msg("UnmarshalYAML")
	if value.Tag != "!!map" {
		return fmt.Errorf("Unable to unmarshal yaml: value not map (%s)", value.Tag)
	}

	for i, node := range value.Content {
		log.Trace().Interface("node1", node).Msg("")
		switch node.Value {
		case "name":
			pkg.Name = value.Content[i+1].Value
		case "version":
			pkg.Version = value.Content[i+1].Value
		case "present":
			pkg.Present, err = strconv.ParseBool(value.Content[i+1].Value)
			if err != nil {
				log.Error().Err(err).Msg("can't parse installed field")
				return err
			}
		}
	}

	log.Trace().Interface("pkg", pkg).Msg("what's in the box?!?!")
	*p = *pkg

	return nil
}

// true/false whether a package is installed
// err = nil if we know what distro we are on
func (p *Package) IsInstalled() (bool, error) {
	log.Trace().Interface("Package", p).Msg("pkgInstalled")
	log.Trace().Interface("Facts", facts.Facts).Msg("what are the facts?")
	if facts.Facts.Distro.Family == "alpine" {
		cmd := exec.Command("apk", "policy", p.Name)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to Cmd.run apk policy")
		}
		stdOut := out.String()
		log.Debug().Str("stdout", stdOut).Msg("stdout")
		if stdOut != "" {
			// package is installed, check version, etc
			if p.Version != "" {
				if strings.Contains(stdOut, p.Version) {
					return true, nil
				} else {
					return false, nil
				}
			}
			return true, nil
		}
		return false, nil
	}
	return false, fmt.Errorf("Unknown distro")
}

func (p *Package) Install() (version string, err error) {
	switch facts.Facts.Distro.Family {
	case "alpine":
		// setting versions on alpine is probably not something most people will be into
		// since it's pretty useless with the base repo's, but we'll support it anyways
		log.Debug().Msgf("Installing on alpine: %s (%s)", p.Name, p.Version)
		// the name and version get smooshed together for the exec
		// i.e. apk add micro~=2
		nameVer := fmt.Sprintf("%s%s", p.Name, p.Version)
		cmd := exec.Command("apk", "add", nameVer)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to cmd.Run apk add")
		}
		log.Debug().Str("stdout", out.String())
		// TODO set version for return
	case "debian":
		log.Debug().Msgf("Installing on debian/ubuntu: %s (%s)", p.Name, p.Version)

	default:
		log.Info().Msgf("Don't know how to install packages on distro: %s", facts.Facts.Distro.Family)
	}
	return "", nil
}

func (p *Package) Ensure(pretend bool) {
	installed, err := p.IsInstalled()
	if err != nil {
		log.Debug().Err(err).Bool("pkg", installed).Msg("")
	}
	if pretend {
		if installed {
			log.Info().Msgf("Package already installed: %s (%s)", p.Name, p.Version)
		} else {
			log.Info().Msgf("Package would be installed: %s (%s)", p.Name, p.Version)
		}
	} else {
		if installed {
			log.Debug().Msgf("Package already installed: %s (%s)", p.Name, p.Version)
		} else {
			// this is the only spot we actually have to do anything other than log
			log.Debug().Msgf("Package being installed: %s (%s)", p.Name, p.Version)
			vers, err := p.Install()
			if err != nil {
				log.Fatal().Err(err).Msgf("Failed to pkg.Install(): %#v", p)
			}
			log.Debug().Msgf("Package installed with version: %s", vers)
		}
	}
}

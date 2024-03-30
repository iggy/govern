// Copyright © 2023 Iggy <iggy@theiggy.com>
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

// TODO handle stopped desired state

package laws

import (
	"bytes"
	"io"
	"os/exec"

	"github.com/iggy/govern/pkg/facts"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Service - package info
type Service struct {
	// Name       string
	State      string `yaml:",omitempty"`
	Persistent bool   `yaml:",omitempty"`
	RunLevel   string `yaml:",omitempty"`

	// CommonFields
	Name   string
	Before []string
	After  []string
}

func (s *Service) UnmarshalYAML(value *yaml.Node) error {
	// log.Logger = log.With().Str("unmarshalyaml", "mount").Logger()

	s.State = "started"
	s.Persistent = true
	s.RunLevel = "default"

	type raw Service
	err := value.Decode((*raw)(s)) // this goes into an infinite loop
	if err != nil && err != io.EOF {
		log.Error().Err(err).Msg("failed to decode yaml")
		return err
	}
	return nil
}

// UnmarshalYAML - This fills in default values if they aren't specified
// func (s *Service) UnmarshalYAML(value *yaml.Node) error {
// 	// defaults
// 	s.State = "enabled"
// 	s.Persistent = true
// 	s.RunLevel = "default"

// 	log.Trace().Interface("Node", value).Msg("UnmarshalYAML Service")
// 	if value.Tag != "!!map" {
// 		return fmt.Errorf("unable to unmarshal yaml: value not map (%s)", value.Tag)
// 	}
// 	for i, node := range value.Content {
// 		log.Trace().Interface("node1", node).Msg("user unmarshal")
// 		switch node.Value {
// 		case "state":
// 			s.State = value.Content[i+1].Value
// 		case "persistent":
// 			s.Persistent, _ = strconv.ParseBool(value.Content[i+1].Value)
// 		case "run_level":
// 			s.RunLevel = value.Content[i+1].Value
// 		}
// 	}
// 	log.Trace().Interface("service", s).Msg("UnmarshalYAML service")

// 	return nil
// }

// CurrentState - get current state of service
func (s *Service) CurrentState() string {
	log.Debug().Str("distro family", facts.Facts.Distro.Family).Str("service", s.Name).Msg("checking service state")
	switch facts.Facts.Distro.Family {
	case "alpine":
		cmd := exec.Command("rc-service", s.Name, "status")
		var out bytes.Buffer
		cmd.Stdout = &out
		// FIXME return 3 just means it's stopped not that anything is wrong, but we should check other failure modes
		_ = cmd.Run()
		// err := cmd.Run()
		// if err != nil {
		// 	log.Fatal().Err(err).Msg("Failed to cmd.Run rc-service status")
		// }
		log.Debug().Str("stdout", out.String())
		switch cmd.ProcessState.ExitCode() {
		case 3:
			return "stopped"
		case 0:
			return "started"
		}
	case "debian":

	default:
		log.Info().Msgf("Don't know how to handle services on distro: %s", facts.Facts.Distro.Family)
	}
	return ""
}

// FIXME changing the runlevel doesn't update the service
// Ensure - ensure service is in desired state
func (s *Service) Ensure(pretend bool) error {
	log.Debug().Str("service name", s.Name).Msg("Service ensure")
	cstate := s.CurrentState()
	if pretend {
		if cstate != s.State {
			log.Info().Str("service name", s.Name).Str("current state", cstate).Str("desired state", s.State).Msg("service not in desired state")
		} else {
			log.Info().Str("service name", s.Name).Str("current state", cstate).Str("desired state", s.State).Msg("service in desired state")
		}
	} else {
		if cstate != s.State {
			if s.State == "started" {
				log.Info().Str("name", s.Name).Str("current state", cstate).Str("desired state", s.State).Msg("starting service")
				switch facts.Facts.Distro.Family {
				case "alpine":
					cmd := exec.Command("rc-service", s.Name, "start")
					var out bytes.Buffer
					cmd.Stdout = &out
					err := cmd.Run()
					if err != nil {
						log.Fatal().Err(err).Msg("Failed to cmd.Run rc-service start")
					}
					log.Debug().Str("stdout", out.String())

					if s.Persistent {
						cmd := exec.Command("rc-update", "add", s.Name, s.RunLevel)
						var out bytes.Buffer
						cmd.Stdout = &out
						err := cmd.Run()
						if err != nil {
							log.Fatal().Err(err).Str("service", s.Name).Msg("Failed to cmd.Run rc-update add")
						}
					}
				case "debian":

				}
			}
		} else {
			log.Debug().Msg("service in desired state")
		}
		if s.Persistent {
			switch facts.Facts.Distro.Family {
			case "alpine":
				// this command is
				cmd := exec.Command("rc-update", "add", s.Name, s.RunLevel)
				var out bytes.Buffer
				cmd.Stdout = &out
				err := cmd.Run()
				if err != nil {
					log.Fatal().Err(err).Msg("Failed to cmd.Run rc-update add")
				}
				log.Debug().Str("stdout", out.String())
			case "debian":
			}

		}
	}
	return nil
}

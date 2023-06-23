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
	"os/exec"

	"github.com/iggy/govern/pkg/facts"
	"github.com/rs/zerolog/log"
)

// Service - package info
type Service struct {
	Name       string
	State      string `yaml:",omitempty"`
	Persistent bool   `yaml:",omitempty"`
	Runlevel   string `yaml:",omitempty"`

	CommonFields
}

// CurrentState - get current state of service
func (s *Service) CurrentState() string {
	log.Debug().Str("distro family", facts.Facts.Distro.Family).Str("service", s.Name).Msg("checking service state")
	switch facts.Facts.Distro.Family {
	case "alpine":
		cmd := exec.Command("rc-service", s.Name, "status")
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to cmd.Run rc-service status")
		}
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

// Ensure - ensure service is in desired state
func (s *Service) Ensure(pretend bool) {
	cstate := s.CurrentState()
	if pretend {
		if cstate != s.State {
			log.Info().Str("current state", cstate).Str("desired state", s.State).Msg("Service not in desired state")
		} else {
			log.Info().Msgf("service in desired state")
		}
	} else {
		if cstate != s.State {
			if s.State == "started" {
				log.Info().Str("current state", cstate).Str("desired state", s.State).Msg("starting service")
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
				cmd := exec.Command("rc-update", "add", s.Name, s.Runlevel)
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
}

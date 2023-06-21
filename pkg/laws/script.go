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

package laws

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Script struct {
	Name       string
	Shell      string
	Script     string
	Env        []string
	Args       []string
	WorkingDir string
	Creates    []string
	RunAs      string
	CommonFields
}

func (s *Script) UnmarshalYAML(value *yaml.Node) error {
	s.Shell = "/bin/sh"
	// TODO
	//  env should match parent shell by default and then be added to
	//

	log.Trace().Interface("Node", value).Msg("UnmarshalYAML")
	if value.Tag != "!!map" {
		return fmt.Errorf("unable to unmarshal yaml: value not map (%s)", value.Tag)
	}

	for i, node := range value.Content {
		log.Trace().Interface("node1", node).Msg("")
		switch node.Value {
		case "name":
			s.Name = value.Content[i+1].Value
		case "shell":
			s.Shell = value.Content[i+1].Value
		case "script":
			s.Script = value.Content[i+1].Value
		case "creates":
			log.Trace().Interface("node2", value.Content[i+1].Content).Msg("")
			for _, v := range value.Content[i+1].Content {
				s.Creates = append(s.Creates, v.Value)
			}
		case "env":
			log.Trace().Interface("node2", value.Content[i+1].Content).Msg("")
		case "args":
			log.Trace().Interface("node2", value.Content[i+1].Content).Msg("")
		case "working_dir":
			s.WorkingDir = value.Content[i+1].Value
		case "run_as":
			s.RunAs = value.Content[i+1].Value
		}
	}

	return nil
}

func (s *Script) Run(pretend bool) error {
	log.Trace().Interface("script", s).Msg("script run")

	if pretend {
		log.Info().Str("script", s.Script).Str("shell", s.Shell).Interface("s", s).Msg("Would run script")
	} else {
		log.Debug().Str("script", s.Script).Str("shell", s.Shell).Interface("s", s).Msg("Running script")

		for _, crts := range s.Creates {
			stat, err := os.Stat(crts)
			if err == nil {
				log.Debug().Str("creates", crts).Interface("stat", stat).Err(err).Msg("creates file already exists")
				return nil
			}
		}

		// check if the script is a URL and download if so
		_, err := url.ParseRequestURI(s.Script)
		if err == nil {
			log.Debug().Str("script", s.Script).Msg("script is a URL")
			resp, err := http.Get(s.Script)
			if err != nil {
				log.Warn().Err(err).Msg("could not download script")
				return err
			}
			defer resp.Body.Close()
			outfile, err := os.Create("tmp.sh")
			if err != nil {
				log.Warn().Err(err).Msg("could not create tmp.sh")
				return err
			}
			size, err := io.Copy(outfile, resp.Body)
			if err != nil {
				log.Warn().Err(err).Msg("could not download script")
				return err
			}
			log.Debug().Int64("size", size).Msg("downloaded script")
			if resp.StatusCode > 299 {
				log.Warn().Err(err).Msg("could not download script")
				return err
			}
			s.Script = "tmp.sh"
		}

		var stdOut, stdErr bytes.Buffer

		cmd := exec.Command(s.Shell, s.Script)
		cmd.Stdout = &stdOut
		cmd.Stderr = &stdErr

		if s.RunAs != "" {
			ids := strings.Split(s.RunAs, ":")
			uid, err := strconv.ParseUint(ids[0], 10, 32)
			if err != nil {
				log.Warn().Err(err).Msg("could not convert uid")
				return err
			}
			gid, err := strconv.ParseUint(ids[1], 10, 32)
			if err != nil {
				log.Warn().Err(err).Msg("could not convert gid")
				return err
			}
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uint32(uid),
					Gid: uint32(gid),
				},
			}
		}

		err = cmd.Run()
		if err != nil {
			log.Error().Err(err).Interface("script", s).Msg("failed to run script")
		}
		log.Info().Str("stdErr", stdErr.String()).Interface("script", s).Msg("script stdErr")
		log.Debug().Str("stdOut", stdOut.String()).Interface("script", s).Msg("script stdOut")
	}

	return nil
}

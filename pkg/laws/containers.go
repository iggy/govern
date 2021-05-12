// Copyright © 2021 Iggy <iggy@theiggy.com>
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
	"context"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/rs/zerolog/log"
)

type HealthCheckOpts struct {
	Enabled     bool
	Command     string
	Interval    string
	Retries     int
	StartPeriod string
	Timeout     string
}

type LogOpts struct {
	Driver string // none|json-file|syslog|journald|gelf|fluentd|awslogs|splunk
	Opt    map[string]string
}

type Container struct {
	Name          string
	Image         string
	Running       bool
	Volumes       map[string]string
	Environment   map[string]string
	Labels        []string
	LogDriver     string
	Hostname      string
	Network       string // bridge|none|container:<name|id>|host|<network-name|network-id>
	HealthCheck   HealthCheckOpts
	Privileged    bool
	PublishAll    bool
	Publish       map[string]string
	RestartPolicy string // no|on-failure[:max-retries]|always|unless-stopped

	CommonFields
}

func (c *Container) IsRunning() (bool, error) {
	log.Trace().Msg("Container.Running()")

	ctx := context.Background()

	client, err := docker.NewClientFromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create docker client")
	}

	lOpts := docker.ListContainersOptions{
		Limit:   1,
		Context: ctx,
		Filters: map[string][]string{
			"name": {c.Name},
		},
	}
	l, err := client.ListContainers(lOpts)
	if err != nil {
		log.Warn().Err(err).Msg("failed to get container list")
	}
	for _, cList := range l {
		log.Trace().Interface("container", cList).Msg("container")
		if c.Name == strings.TrimLeft(cList.Names[0], "/") && cList.State == "running" {
			log.Debug().Msgf("Container running: %s", c.Name)
			return true, nil
		}
	}

	return false, nil
}

func (c *Container) Ensure(pretend bool) error {
	running, rErr := c.IsRunning()
	if !running {
		log.Debug().Err(rErr).Msgf("container not running: %s", c.Name)
		if pretend {
			log.Info().Msgf("Container not running, would start: %s", c.Name)
		}
	}
	return nil
}

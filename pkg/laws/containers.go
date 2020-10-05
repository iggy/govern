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
	"context"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
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

	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create docker client")
		return false, err
	}
	log.Trace().Msgf("Docker Client Version: %s", docker.ClientVersion())
	lFilters := filters.NewArgs(filters.Arg("name", c.Name))
	lOpts := types.ContainerListOptions{Filters: lFilters}
	hostContainers, err := docker.ContainerList(context.Background(), lOpts)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list docker containers")
		return false, err
	}

	log.Trace().Interface("cntr", c).Msgf("")
	log.Trace().Interface("hcs", hostContainers).Msgf("")
	// We are filtering by name above, so we should only get one (or 0) results,
	// but we still have to loop over the results
	for _, hc := range hostContainers {
		log.Trace().Interface("container", hc).Msgf("%s %s\n", hc.ID[:10], hc.Image)
		if c.Name == strings.TrimLeft(hc.Names[0], "/") && hc.State == "running" {
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

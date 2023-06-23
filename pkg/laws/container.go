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
	"fmt"
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/yaml.v3"

	"github.com/rs/zerolog/log"
)

// HealthCheckOpts - This is a struct for the healthcheck options
type HealthCheckOpts struct {
	Enabled     bool
	Command     string
	Interval    string
	Retries     int
	StartPeriod string
	Timeout     string
}

// LogOpts - This is a struct for the log options
type LogOpts struct {
	Driver string // none|json-file|syslog|journald|gelf|fluentd|awslogs|splunk
	Opt    map[string]string
}

// Container -  This is a struct for the container
type Container struct {
	Name          string
	Image         string
	Running       bool
	Volumes       map[string]string
	Environment   map[string]string
	Labels        map[string]string
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

// UnmarshalYAML - This fills in default values if they aren't specified
func (c *Container) UnmarshalYAML(value *yaml.Node) error {
	// set default values
	c.Running = true
	c.Labels = map[string]string{"StartedBy": "Govern"}
	c.Privileged = false
	var err error // for use in the switch below

	log.Trace().Interface("Node", value).Interface("node type", value.Content).Msg("Container UnmarshalYAML")
	if value.Tag != "!!map" {
		return fmt.Errorf("unable to unmarshal yaml: value not map (%s)", value.Tag)
	}

	for i, node := range value.Content {
		log.Trace().Interface("node1", node).Msg("")
		switch node.Value {
		case "name":
			log.Trace().Str("key", value.Content[i].Value).Str("value", value.Content[i+1].Value).Msg("name yaml tag")
			c.Name = value.Content[i+1].Value
			if c.Name == "" {
				return nil
			}
		case "image":
			c.Image = value.Content[i+1].Value
		case "running":
			c.Running, err = strconv.ParseBool(value.Content[i+1].Value)
			if err != nil {
				log.Error().Err(err).Msg("can't parse running field")
				return err
			}
		case "labels":
			// for l := range node.
			log.Trace().Interface("node", node).Msg("labels")
		}
	}

	return nil
}

// IsRunning -  This checks if the container is running
func (c *Container) IsRunning() (bool, error) {
	log.Trace().Interface("c", c).Msg("Container.Running()")

	// ctx := context.Background()

	client, err := docker.NewClientFromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create docker client")
	}

	lOpts := docker.ListContainersOptions{
		// Limit:   1,
		// Context: ctx,
		Filters: map[string][]string{
			"name":   {c.Name},
			"status": {"running"},
		},
	}
	l, err := client.ListContainers(lOpts)
	if err != nil {
		log.Warn().Err(err).Msg("failed to get container list")
	}
	for _, cList := range l {
		log.Trace().
			Interface("container", cList).
			Str("name", strings.TrimLeft(cList.Names[0], "/")).
			Str("yaml name", c.Name).
			Msg("container")
		if c.Name == strings.TrimLeft(cList.Names[0], "/") {
			log.Debug().Msgf("Container running: %s", c.Name)
			return true, nil
		}
	}

	return false, nil
}

// Ensure - run the container if it isn't running
func (c *Container) Ensure(pretend bool) error {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		log.Error().Err(err).Msg("failed to create client connection")
	}
	running, rErr := c.IsRunning()
	if !running && c.Running {
		log.Debug().Err(rErr).Msgf("container not running: %s", c.Name)
		if pretend {
			log.Info().Msgf("Container not running, would start: %s", c.Name)
		} else {
			if !isImagePulled(*client, c.Image) {
				log.Info().Str("image", c.Image).Msg("image doesn't exist, pulling")
				imageData := strings.Split(c.Image, ":")
				pullImageOpts := docker.PullImageOptions{
					Repository: imageData[0],
					Tag:        imageData[1],
				}
				err := client.PullImage(pullImageOpts, docker.AuthConfiguration{})
				if err != nil {
					log.Error().Err(err).Str("image", c.Image).Msg("failed to pull image")
				}
			}

			if !isContainerCreated(*client, c.Name) {
				log.Trace().Msg("container doesn't exist, creating")
				createContainerOpts := docker.CreateContainerOptions{
					Name: c.Name,
					Config: &docker.Config{
						Labels: c.Labels,
						Image:  c.Image,
						// Volumes: c.Volumes,
						// Healthcheck: c.HealthCheck,
					},
				}

				cnt, err := client.CreateContainer(createContainerOpts)
				if err != nil {
					log.Error().Err(err).Interface("container", cnt).Msg("failed to create container")
				}
			}
			running, err := c.IsRunning()
			if err != nil {
				log.Error().Err(err).Msg("failed to see if container running")
			}
			if !running {
				err := client.StartContainer("ffd57aa8a6ea1bf28e9ab00114e7c9dd36f2edeee763a67f18d9f76062cec33d", &docker.HostConfig{})
				if err != nil {
					log.Error().Err(err).Msg("failed to start container")
				}
			}

		}
	}
	return nil
}

func isImagePulled(dc docker.Client, name string) bool {
	listImagesOpts := docker.ListImagesOptions{
		Filter: name,
	}
	imgs, err := dc.ListImages(listImagesOpts)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	for _, i := range imgs {
		log.Trace().Str("i.ID", i.ID).Str("i.ParentID", i.ParentID).Interface("i", i).Msg("img")
		// >1 is something I've never seen, <1 means the image isn't tagged
		if len(i.RepoTags) != 1 {
			if len(i.RepoTags) > 1 {
				log.Warn().Strs("RepoTags", i.RepoTags).Msg("unexpected RepoTags length")
			}
			return false
		}
		if i.RepoTags[0] == name {
			return true
		}
	}
	return false
}

func isContainerCreated(dc docker.Client, name string) bool {
	listContainersOpts := docker.ListContainersOptions{
		Filters: map[string][]string{
			// 	"name": {name},
			"status": {"created", "restarting", "paused", "exited", "dead"},
		},
	}
	l, err := dc.ListContainers(listContainersOpts)
	if err != nil {
		log.Warn().Err(err).Msg("failed to get container list")
	}
	log.Trace().Interface("container list", l).Msg("")
	for _, cList := range l {
		log.Trace().
			Interface("container", cList).
			Str("name", strings.TrimLeft(cList.Names[0], "/")).
			Str("yaml name", name).
			Msg("isContainerCreated")
		if name == strings.TrimLeft(cList.Names[0], "/") {
			log.Debug().Msgf("Container exists: %s", name)
			return true
		}
	}

	return false
}

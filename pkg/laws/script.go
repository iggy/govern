package laws

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Script struct {
	Name       string
	Shell      string
	Script     string
	Env        []string
	WorkingDir string
	CommonFields
}

func (s *Script) UnmarshalYAML(value *yaml.Node) error {
	s.Shell = "/bin/sh"

	log.Trace().Interface("Node", value).Msg("UnmarshalYAML")
	if value.Tag != "!!map" {
		return fmt.Errorf("Unable to unmarshal yaml: value not map (%s)", value.Tag)
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

		}
	}

	return nil
}

func (s *Script) Run(pretend bool) error {
	log.Trace().Interface("script", s).Msg("script run")

	if pretend {
		log.Info().Str("script", s.Script).Str("shell", s.Shell).Msg("Would run script")
	} else {
		log.Debug().Str("script", s.Script).Str("shell", s.Shell).Msg("Running script")

		var stdOut, stdErr bytes.Buffer

		cmd := exec.Command(s.Shell, "-c", s.Script)
		cmd.Stdout = &stdOut
		cmd.Stderr = &stdErr

		err := cmd.Run()
		if err != nil {
			log.Error().Err(err).Interface("script", s).Msg("failed to run script")
		}
		log.Info().Str("stdErr", stdErr.String()).Interface("script", s).Msg("script stdErr")
		log.Debug().Str("stdOut", stdOut.String()).Interface("script", s).Msg("script stdOut")
	}

	return nil
}

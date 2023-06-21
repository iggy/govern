package laws

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Mount struct {
	Spec       string
	MountPoint string
	Type       string
	Options    string
	Freq       int64
	Pass       int64

	CommonFields
}

func (m *Mount) UnmarshalYAML(value *yaml.Node) error {
	// m & Mount{}
	m.Freq = 0
	m.Pass = 0
	m.Options = "defaults"

	log.Trace().Interface("Node", value).Msg("UnmarshalYAML")
	if value.Tag != "!!map" {
		return fmt.Errorf("Unable to unmarshal yaml: value not map (%s)", value.Tag)
	}

	for i, node := range value.Content {
		log.Trace().Interface("node1", node).Msg("")
		switch node.Value {
		case "spec":
			m.Spec = value.Content[i+1].Value
		case "mount-point":
			m.MountPoint = value.Content[i+1].Value
		case "type":
			m.Type = value.Content[i+1].Value
		case "options":
			m.Options = value.Content[i+1].Value
		case "freq":
			m.Freq, _ = strconv.ParseInt(value.Content[i+1].Value, 10, 64)
		case "pass":
			m.Pass, _ = strconv.ParseInt(value.Content[i+1].Value, 10, 64)
			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
		}
	}
	log.Trace().Interface("m", m).Msg("what's in the box?!?!")

	return nil

}

func (m *Mount) Ensure(pretend bool) error {
	present, err := m.Exists()
	if err != nil {
		log.Debug().Err(err).Bool("mount", present).Msg("")
	}
	if pretend {
		if present {
			log.Info().Msgf("mount already setup: %s (%s)", m.Spec, m.MountPoint)
		} else {
			log.Info().Msgf("would add mount: %s (%s)", m.Spec, m.MountPoint)
		}
	} else {
		if present {
			log.Debug().Msgf("mount already setup: %s (%s)", m.Spec, m.MountPoint)
		} else {
			// this is the only spot we actually have to do anything other than log
			log.Debug().Msgf("mount being setup: %s (%s)", m.Spec, m.MountPoint)
			// vers, err := p.Install()
			// if err != nil {
			// log.Fatal().Err(err).Msgf("Failed to pkg.Install(): %#v", p)
			// }
			// log.Debug().Msgf("Package installed with version: %s", vers)
			fstabLine := fmt.Sprintf("%s\t%s\t%s\t%s\t%d %d\n", m.Spec, m.MountPoint, m.Type, m.Options, m.Freq, m.Pass)
			f, err := os.OpenFile("/etc/fstab", os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to open fstab")

			}
			defer f.Close()
			if _, err := f.WriteString(fstabLine); err != nil {
				log.Fatal().Err(err).Msg("failed to write mountpoint to fstab")
			}
		}
	}

	return nil
}

func (m *Mount) Exists() (bool, error) {
	lines, err := os.ReadFile("/etc/fstab")
	if err != nil {
		log.Debug().Err(err).Msg("failed to read fstab")
		return false, err
	}
	// for _, l := range lines {
	// 	log.Trace().Str("line", l).Msg("")
	// }
	log.Trace().Bytes("lines", lines).Str("spec", m.Spec).Msg("checking if mountpoint exists")
	if bytes.Contains(lines, []byte(m.Spec)) {
		return true, nil
	}
	return false, nil
}

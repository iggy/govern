package laws

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Mount is a mount point
type Mount struct {
	// Name       string
	Spec       string
	MountPoint string `yaml:"mount_point"`
	Type       string
	Options    string
	Freq       int64
	Pass       int64
	Present    bool

	// CommonFields
	Name   string
	Before []string
	After  []string
}
type AbsentMount struct {
	// Name       string
	Spec       string
	MountPoint string `yaml:"mount_point"`
	Type       string
	Options    string
	Freq       int64
	Pass       int64

	// CommonFields
	Name   string
	Before []string
	After  []string
}

// UnmarshalYAML implements the Unmarshaler interface
func (m *Mount) UnmarshalYAML(value *yaml.Node) error {
	// log.Logger = log.With().Str("unmarshalyaml", "mount").Logger()

	m.Freq = 0
	m.Pass = 0
	m.Options = "defaults"
	m.Present = true

	// var tmp interface{}
	// var newM Mount
	type rawMount Mount
	// err := yaml.NewDecoder(strings.NewReader(value.Value)).Decode(&newM)
	err := value.Decode((*rawMount)(m)) // this goes into an infinite loop
	if err != nil && err != io.EOF {
		log.Error().Err(err).Msg("failed to decode yaml")
	}

	// log.Trace().
	// 	Interface("Node", value).
	// 	Interface("newM", m).
	// 	Interface("content", value.Content).
	// 	Str("value", value.Value).
	// 	Str("shorttag", value.ShortTag()).
	// 	Str("longtag", value.LongTag()).
	// 	Str("anchor", value.Anchor).
	// 	Msgf("%v", value.Value)
	// if value.Tag != "!!map" {
	// 	return fmt.Errorf("unable to unmarshal yaml: value not map (%s)", value.Tag)
	// }

	// for i, node := range value.Content {
	// 	log.Trace().Interface("node1", node).Msg("")
	// 	switch node.Value {
	// 	case "name":
	// 		m.Name = value.Content[i+1].Value
	// 	case "spec":
	// 		m.Spec = value.Content[i+1].Value
	// 	case "mount-point":
	// 		m.MountPoint = value.Content[i+1].Value
	// 	case "type":
	// 		m.Type = value.Content[i+1].Value
	// 	case "options":
	// 		m.Options = value.Content[i+1].Value
	// 	case "freq":
	// 		m.Freq, _ = strconv.ParseInt(value.Content[i+1].Value, 10, 64)
	// 	case "pass":
	// 		m.Pass, _ = strconv.ParseInt(value.Content[i+1].Value, 10, 64)
	// 		// common fields
	// 	case "after":
	// 		for _, j := range value.Content[i+1].Content {
	// 			m.After = append(m.After, j.Value)
	// 		}
	// 	case "before":
	// 		for _, j := range value.Content[i+1].Content {
	// 			m.Before = append(m.Before, j.Value)
	// 		}

	// 		// case "":
	// 		// m. = value.Content[i+1].Value
	// 		// case "":
	// 		// m. = value.Content[i+1].Value
	// 		// case "":
	// 		// m. = value.Content[i+1].Value
	// 		// case "":
	// 		// m. = value.Content[i+1].Value
	// 	}
	// }
	// log.Trace().Interface("m", m).Msg("what's in the box?!?!")

	// os.Exit(0)

	return nil

}
func (m *AbsentMount) UnmarshalYAML(value *yaml.Node) error {
	// log.Logger = log.With().Str("unmarshalyaml", "mount").Logger()

	m.Freq = 0
	m.Pass = 0
	m.Options = "defaults"

	type rawMount AbsentMount
	err := value.Decode((*rawMount)(m)) // this goes into an infinite loop
	if err != nil && err != io.EOF {
		log.Error().Err(err).Msg("failed to decode yaml")
		return err
	}
	return nil
}

// Ensure - ensure mount is setup
// TODO should probably mark fstab as managed by govern
func (m *Mount) Ensure(pretend bool) error {
	exists, err := m.Exists()
	if err != nil {
		log.Debug().Err(err).Bool("mount", exists).Msg("")
	}
	if pretend {
		if m.Present {
			if exists {
				log.Info().Msgf("mount already setup: %s (%s)", m.Spec, m.MountPoint)
			} else {
				log.Info().Msgf("would add mount: %s (%s)", m.Spec, m.MountPoint)
			}
		} else {
			if exists {
				log.Info().Str("mountpoint", m.MountPoint).Str("spec", m.Spec).Msg("mount exists, but shouldn't, removing")
			}
		}
	} else {
		if exists {
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
				log.Error().Err(err).Msg("failed to open fstab")

			}
			defer f.Close()
			if _, err := f.WriteString(fstabLine); err != nil {
				log.Error().Err(err).Msg("failed to write mountpoint to fstab")
			}
		}
	}

	return nil
}
func (m *AbsentMount) Ensure(pretend bool) error {
	exists, err := m.Exists()
	if err != nil {
		log.Debug().Err(err).Bool("mount", exists).Msg("")
	}
	if pretend {
		if exists {
			log.Info().Str("mountpoint", m.MountPoint).Str("spec", m.Spec).Msg("mount exists, but shouldn't, removing")
		}
	} else {
		if exists {
			log.Debug().Str("spec", m.Spec).Msg("mount absent unimpl")
		}
	}

	return nil
}

// Exists - check if mountpoint exists
func (m *Mount) Exists() (bool, error) {
	return _exists(m.Spec)
}
func (m *AbsentMount) Exists() (bool, error) {
	return _exists(m.Spec)
}

func _exists(spec string) (bool, error) {
	lines, err := os.ReadFile("/etc/fstab")
	if err != nil {
		log.Debug().Err(err).Msg("failed to read fstab")
		return false, err
	}
	// for _, l := range lines {
	// 	log.Trace().Str("line", l).Msg("")
	// }
	log.Trace().Bytes("lines", lines).Str("spec", spec).Msg("checking if mountpoint exists")
	if bytes.Contains(lines, []byte(spec)) {
		return true, nil
	}
	return false, nil
}

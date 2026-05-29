package laws

import (
	"bytes"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
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
func (m *Mount) UnmarshalYAML(unmarshal func(interface{}) error) error {
	m.Freq = 0
	m.Pass = 0
	m.Options = "defaults"
	m.Present = true

	type rawMount Mount
	if err := unmarshal((*rawMount)(m)); err != nil {
		log.Error().Err(err).Msg("failed to decode yaml")
		return err
	}
	return nil
}

func (m *AbsentMount) UnmarshalYAML(unmarshal func(interface{}) error) error {
	m.Freq = 0
	m.Pass = 0
	m.Options = "defaults"

	type rawMount AbsentMount
	if err := unmarshal((*rawMount)(m)); err != nil {
		log.Error().Err(err).Msg("failed to decode yaml")
		return err
	}
	return nil
}

// Ensure - ensure mount is setup
// TODO
//
//	should probably mark fstab as managed by govern
//	create mountpoint if it doesn't exist
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
			// TODO make the dir
			// this is the only spot we actually have to do anything other than log
			log.Debug().Msgf("mount being setup: %s (%s)", m.Spec, m.MountPoint)
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

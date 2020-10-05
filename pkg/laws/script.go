package laws

import (
	"github.com/rs/zerolog/log"
)

type Script struct {
	Name   string
	Shell  string
	Script string
	CommonFields
}

func (s *Script) Run() error {
	log.Trace().Interface("script", s).Msg("script run")

	return nil
}

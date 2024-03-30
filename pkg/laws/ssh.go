package laws

import (
	"bufio"
	"errors"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

type SSHKey struct {
	Name   string
	Key    string
	User   string
	Before []string
	After  []string
}

func (k *SSHKey) Ensure(pretend bool) error {
	fl := log.With().Str("key name", k.Name).Logger() // function logger adds some extra info

	if pretend {
		fl.Info().Str("user", k.User).Msg("adding authorized_key")
		return nil
	}

	u, err := user.Lookup(k.User)
	if err != nil {
		fl.Error().Err(err).Msg("failed to lookup user")
		return err
	}
	userSSHDir := path.Join(u.HomeDir, ".ssh")
	authKeyPath := path.Join(userSSHDir, "authorized_keys")
	_, err = os.Stat(userSSHDir)
	if err != nil {
		fl.Debug().Err(err).Msg("failed to stat ~/.ssh")
		if errors.Is(err, os.ErrNotExist) {
			// ~/.ssh doesn't exist, create it
			err = os.MkdirAll(userSSHDir, 0700)
			if err != nil {
				fl.Error().Err(err).Msg("failed to make ~/.ssh")
			}
			uid, err := strconv.ParseInt(u.Uid, 10, 32)
			if err != nil {
				fl.Error().Err(err).Msg("failed to parse uid")
			}
			gid, err := strconv.ParseInt(u.Gid, 10, 32)
			if err != nil {
				fl.Error().Err(err).Msg("failed to parse gid")
			}
			err = os.Chown(userSSHDir, int(uid), int(gid))
			if err != nil {
				log.Error().Err(err).Msg("failed to chown ~/.ssh")
			}
		} else {
			return err
		}
	}
	fr, err := os.Open(authKeyPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fl.Error().Err(err).Msg("failed to open authorized_keys file for reading")
			return err
		} else {
			// TODO fix this logic to reduce duplication
			fw, err := os.OpenFile(authKeyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				fl.Error().Err(err).Msg("failed to open authorized_keys file for writing")
				return err
			}
			_, err = fw.WriteString(k.Key + "\n")
			if err != nil {
				log.Error().Err(err).Msg("failed to write key to auth_keys")
			}
			defer fw.Close()
			// TODO chown file if necessary

			return nil
		}
	}
	scanner := bufio.NewScanner(fr)
	// var newContent []string
	keyFound := false
	for scanner.Scan() {
		line := scanner.Text()
		fl.Info().Str("line", line).Msg("")
		if strings.Contains(line, k.Key) {
			keyFound = true
			continue
		}
	}
	fr.Close()

	if !keyFound {
		fw, err := os.OpenFile(authKeyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			fl.Error().Err(err).Msg("failed to open authorized_keys file for writing")
			return err
		}
		_, err = fw.WriteString(k.Key + "\n")
		if err != nil {
			log.Error().Err(err).Msg("failed to write key to auth_keys")
		}
		defer fw.Close()
		// TODO chown file if necessary
	}

	return nil
}

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
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/iggy/govern/pkg/facts"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// User - a user the system should have
type User struct {
	Name           string   ``                       // the user's name
	UID            uint64   `yaml:",omitempty"`      // the user's UID, uint64 matches
	GID            uint64   `yaml:",omitempty"`      // The primary group ID
	Fullname       string   ``                       // part of the GECOS string
	Password       string   ``                       // the encrypted password
	HomeDir        string   ``                       // the user's $HOME
	Shell          string   ``                       // the system shell
	System         bool     ``                       // whether this is a system user or not
	Exists         bool     ``                       // Whether the user should exist on the system or not
	ExtraGroups    []string ``                       // required extra group names
	OptionalGroups []string `yaml:"optional_groups"` // if these groups exist already, add the user to them, otherwise ignore
	CommonFields            //CommonFields `yaml:"commonfields,inline"` // fields that are supported for everything, mostly dep related
	// TODO *Groups ^ should be array and should be expanded out below
}

// UnmarshalYAML - This fills in default values if they aren't specified
func (u *User) UnmarshalYAML(value *yaml.Node) error {
	var err error      // for use in the switch below
	u.UID = ^uint64(0) // effectively -1, but go does math different than C
	u.GID = ^uint64(0) // see https://blog.golang.org/constants
	// log.Trace().Interface("Node", value).Msg("UnmarshalYAML")
	if value.Tag != "!!map" {
		return fmt.Errorf("Unable to unmarshal yaml: value not map (%s)", value.Tag)
	}
	for i, node := range value.Content {
		// log.Trace().Interface("node1", node).Msg("")
		switch node.Value {
		case "name":
			u.Name = value.Content[i+1].Value
		case "uid":
			u.UID, _ = strconv.ParseUint(value.Content[i+1].Value, 10, 64)
		case "gid":
			u.GID, _ = strconv.ParseUint(value.Content[i+1].Value, 10, 64)
		case "fullname":
			u.Fullname = value.Content[i+1].Value
		case "password":
			u.Password = value.Content[i+1].Value
		case "homedir":
			u.HomeDir = value.Content[i+1].Value
		case "shell":
			u.Shell = value.Content[i+1].Value
		case "system":
			u.System, err = strconv.ParseBool(value.Content[i+1].Value)
			if err != nil {
				log.Error().Err(err).Msg("can't parse system field")
				return err
			}
		case "exists":
			u.Exists, err = strconv.ParseBool(value.Content[i+1].Value)
			if err != nil {
				log.Error().Err(err).Msg("can't parse exists field")
				return err
			}
		case "extra_groups":
			for _, g := range value.Content[i+1].Content {
				u.ExtraGroups = append(u.ExtraGroups, g.Value)
			}
		case "optional_groups":
			for _, g := range value.Content[i+1].Content {
				u.OptionalGroups = append(u.OptionalGroups, g.Value)
			}
		}
	}
	log.Trace().Interface("user", u).Msg("what's in the box?!?!")

	return nil
}

// Common internet wisdom says I should be talking to pam, but Alpine doesn't use pam
func (u *User) GetPassword() (string, error) {
	shadowFile, err := os.Open("/etc/shadow")
	passwd := ""
	if err != nil {
		log.Error().Err(err).Msg("Failed to open shadow file")
		return "", err
	}
	scanner := bufio.NewScanner(shadowFile)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, u.Name) {
			// Facts.Distro.Version = fmt.Sprint(strings.Split(line, "=")[1])
			userLine := strings.Split(line, ":")
			passwd = userLine[1]
		}
	}
	if err = scanner.Err(); err != nil {
		log.Error().Err(err).Msg("failed to find user in shadow file")
		return "", err
	}

	return passwd, nil
}

func (u *User) Create() {
	var args []string
	var cmd *exec.Cmd
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	log.Debug().Msg("Creating user")

	// check which kind of distro we're on
	_, err := os.Stat("/usr/sbin/adduser")
	if err != nil && facts.Facts.Distro.Slug == "alpine" {
		log.Debug().Msg("on Alpine, using adduser")
		// TODO handle optionalgroups, extragroups, system, password
		args = append(args,
			"-u", fmt.Sprintf("%d", u.UID),
			"-s", u.Shell,
			"-G", fmt.Sprintf("%d", u.GID),
			"-D",
			"-g", u.Fullname,
			"-h", u.HomeDir,
		)
		args = append(args, u.Name)
		cmd = exec.Command("adduser", args...)
	} else if facts.Facts.Distro.Family == "debian" {
		log.Debug().Msg("on a debian based system, using useradd")
		args := []string{
			"-u", fmt.Sprintf("%d", u.UID),
			"-s", u.Shell,
			"-g", fmt.Sprintf("%d", u.GID),
			"-c", u.Fullname,
			"-d", u.HomeDir,
			"-p", u.Password,
		}
		args = append(args, u.Name)
		cmd = exec.Command("useradd", args...)
	}
	log.Debug().Strs("args", args).Msg("calling add user command with args")
	cmd.Stdout = &stdOut
	cmd.Stdout = &stdErr
	err = cmd.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create user")
	}
	log.Debug().Str("stdout", stdOut.String()).Str("stderr", stdErr.String()).Msg("add user command output")

}

func (u *User) Ensure(pretend bool) error {
	log.Trace().Interface("user", u).Msgf("ensuring user: %s (%d:%d)", u.Name, u.UID, u.GID)
	eu, err := user.Lookup(u.Name)
	switch err.(type) {
	case user.UnknownUserError:
		if pretend {
			log.Info().Msg("user doesn't exist, creating")
		} else {
			log.Trace().Msg("user doesn't exist, creating")
			u.Create()
		}
	default: // Probably nil (user exists)
		if pretend {
			log.Info().Msg("user exists, updating if necessary")
		}
		log.Debug().Interface("eu", eu).Err(err).Msg("user exists, making sure it matches")
		if eu.Uid != strconv.FormatUint(u.UID, 10) {
			log.Info().Str("existing UID", eu.Uid).Uint64("wanted UID", u.UID).Msg("UID doesn't match, changing")
		}
		if eu.Gid != strconv.FormatUint(u.GID, 10) {
			log.Info().Str("existing GID", eu.Gid).Uint64("wanted GID", u.GID).Msg("GID doesn't match, changing")
		}
		if u.HomeDir != "" && strings.TrimSpace(eu.HomeDir) != strings.TrimSpace(u.HomeDir) {
			log.Info().Str("existing homedir", eu.HomeDir).Str("wanted homedir", u.HomeDir).Msg("homedir doesn't match, changing")
		}
		if u.Fullname != "" && eu.Name != u.Fullname {
			log.Info().Str("existing fullname", eu.Name).Str("wanted fullname", u.Fullname).Msg("fullname doesn't match, changing")
		}
		pwd, perr := u.GetPassword()
		if perr != nil {
			return perr
		}
		if u.Password != "" && pwd != u.Password {
			log.Info().Str("u.Password", u.Password).Str("user", u.Name).Msg("password doesn't match, changing")
		}
		// system field is only interesting when creating new users
		// if eu.System != u.System {
		// log.Debug().Str("eu.System", eu.System).Str("u.System", u.System).Msg("System doesn't match, changing")
		// }
	}
	return nil
}

// Group - a group the system should have
type Group struct {
	Name   string
	GID    uint64
	System bool
	CommonFields
}

// Ensure - check if the group exists
func (g *Group) Ensure(pretend bool) error {
	log.Trace().Msgf("Group.Ensure(): %s", g.Name)
	grp, err := user.LookupGroup(g.Name)
	// log.Debug().Interface("grp", grp).Interface("g", g).Str("g.gid", fmt.Sprintf("%d", g.GID)).Str("grp.gid", grp.Gid).Msg("grp lookup")
	switch err.(type) {
	case user.UnknownGroupError:
		// group doesn't exist, create it
		if pretend {
			log.Info().Msgf("group will be created: %s", g.Name)
		} else {
			err = g.Create()
			if err != nil {
				log.Error().Err(err).Msg("failed to create group")
				return err
			}
		}
	case nil:
		// group exists, check it
		if grp.Gid == fmt.Sprintf("%d", g.GID) {
			if pretend {
				log.Info().Msgf("group exists: %s", g.Name)
			}
		} else {
			log.Debug().Msg("group exists, but GID doesn't match")
		}
	default:
		// some other kind of error
		log.Error().Err(err).Msg("failed to lookup group")
		return err
	}
	log.Trace().Msgf("group group: %#v - %#v", g, grp)
	return nil
}

// Create - create a group
func (g *Group) Create() error {
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer
	var args []string
	var cmd *exec.Cmd

	log.Trace().Msgf("Group.Create(): %s", g.Name)
	switch facts.Facts.Distro.Family {
	case "alpine":
		args = append(args, "-g", fmt.Sprintf("%d", g.GID))
		if g.System {
			args = append(args, "-S")
		}
		args = append(args, g.Name)
		log.Trace().Interface("args", args).Msg("group create() args")
		cmd = exec.Command("addgroup", args...)
	case "debian":
		args := []string{"-g", fmt.Sprintf("%d", g.GID)}
		if g.System {
			args = append(args, "-r")
		}
		args = append(args, g.Name)
		log.Trace().Interface("args", args).Msg("group create() args")
		cmd = exec.Command("groupadd", args...)
	}
	log.Debug().Strs("args", args).Msg("calling add group command with args")

	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create group")
	}
	log.Debug().
		Str("stdout", stdOut.String()).
		Str("stderr", stdErr.String()).
		Msg("add group command output")

	return nil
}

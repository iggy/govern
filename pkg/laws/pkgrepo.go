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
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/iggy/govern/pkg/facts"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// PackageRepo describes a package repository
type PackageRepo struct {
	// Name     string
	Key      string `yaml:"key"` // (gpg|etc) key to fetch and load into the system store
	Contents string // the repo URL usually
	// CommonFields
	Name   string // unique identifier, not used in the actual repo
	Before []string
	After  []string
}

// UnmarshalYAML implements the Unmarshaler interface
func (r *PackageRepo) UnmarshalYAML(value *yaml.Node) error {
	// var err error // for use in the switch below

	// repo := &PackageRepo{}
	// repo.Present = true

	log.Trace().Interface("Node", value).Msg("PackageRepo UnmarshalYAML")
	if value.Tag != "!!map" {
		return fmt.Errorf("unable to unmarshal yaml: value not map (%s)", value.Tag)
	}

	for i, node := range value.Content {
		log.Trace().Interface("node1", node).Msg("")
		switch node.Value {
		case "name":
			r.Name = value.Content[i+1].Value
		case "key":
			r.Key = value.Content[i+1].Value
		case "contents":
			r.Contents = value.Content[i+1].Value
		case "before":
			for _, j := range value.Content[i+1].Content {
				r.Before = append(r.Before, j.Value)
			}
		case "after":
			for _, j := range value.Content[i+1].Content {
				r.After = append(r.After, j.Value)
			}
			// case "present":
			// 	r.Present, err = strconv.ParseBool(value.Content[i+1].Value)
			// 	if err != nil {
			// 		log.Error().Err(err).Msg("can't parse installed field")
			// 		return err
			// 	}
		}
	}

	// *r = *repo
	return nil
}

func (r *PackageRepo) Ensure(pretend bool) error {
	switch facts.Facts.Distro.Family {
	case "alpine":
		isitin, err := lineInFile(r.Contents, "/etc/apk/repositories")
		if err != nil {
			log.Error().Err(err).Msg("alpine package repo: couldn't check existing repo config")
		}
		if !isitin {
			if pretend {
				log.Info().Str("name", r.Name).Str("contents", r.Contents).Msg("adding package repo")
			} else {
				// first lets handle the key
				// TODO should we check if it exists already?
				if r.Key == "" {
					log.Error().Interface("pkgrepo", r).Msg("key isn't set")
					return fmt.Errorf("pkgrepo key isn't set: %s", r.Name)
				}
				c := &http.Client{}
				resp, err := c.Get(r.Key)
				if err != nil {
					log.Error().Err(err).Str("key", r.Key).Msg("get: failed to get gpg key")
				}
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Error().Err(err).Str("key", r.Key).Msg("read: failed to get gpg key")
				}
				gpgSplit := strings.Split(r.Key, "/")
				outfileName := gpgSplit[len(gpgSplit)-1]
				outfilePath := path.Join("/etc/apk/keys", outfileName)
				err = os.WriteFile(outfilePath, body, 0755)
				if err != nil {
					log.Error().Err(err).Str("key", r.Key).Msg("failed to write gpg key")
				}

				// now add the repo url to /etc/apk/repositories
				ear, err := os.OpenFile("/etc/apk/repositories", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Error().Err(err).Str("contents", r.Contents).Msg("failed to open /e/a/r")

				}
				_, err = ear.Write(bytes.NewBufferString(r.Contents + "\n").Bytes())
				if err != nil {
					log.Error().Err(err).Str("contents", r.Contents).Msg("failed to write to /e/a/r")
				}
				// TODO run update after adding repo
			}
		}
	case "debian":
		// should we try add-apt-repo first and then fallback to the manual way?
	}
	return nil
}

func lineInFile(line, file string) (bool, error) {
	f, err := os.Open(file)
	if err != nil {
		log.Error().Err(err).Str("file", file).Str("line", line).Msg("failed to open file for scanning")
		return false, err
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() == line {
			return true, nil
		}
	}
	return false, nil
}

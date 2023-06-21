// Copyright © 2023 Iggy <iggy@theiggy.com>
//
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
	"io/fs"
	"io/ioutil"
	"os"

	// "gopkg.in/yaml.v3"
	"github.com/rs/zerolog/log"
)

type File struct {
	Path   string
	User   string
	Group  string
	Mode   fs.FileMode
	Text   string
	Backup bool
}

// func (f *File) UnmarshalYAML(value *yaml.Node) error {
//     log.Trace().Msg("file unmarshall yaml")

//     return nil
// }

func (f *File) Ensure(pretend bool) error {
	log.Trace().Interface("File", f).Msg("file ensure")

	if pretend {
		if f.Exists() {
			log.Debug().Msg("file exists, skipping")
		} else {
			log.Debug().Msg("file doesn't exist, would create")
		}
	} else {
		// TODO make sure we aren't overwriting an existing file
		if f.Backup && f.Exists() {
			log.Trace().Msg("backing up file before writing")
			err := os.Rename(f.Path, f.Path+".bak")
			if err != nil {
				log.Error().Err(err).Interface("file", f).Msg("failed to backup file")
			}
		}
		if !f.Exists() {
			// we just always write the file, opening -< checking -> possibly writing is often slower than just writing
			err := ioutil.WriteFile(f.Path, []byte(f.Text), f.Mode)
			if err != nil {
				log.Error().Err(err).Interface("File", f).Msg("failed to write file")
			}
		} else {
			log.Trace().Msg("updating file to match")
		}
		// ->checking -> possibly writing is often slower than just writing
		err := ioutil.WriteFile(f.Path, []byte(f.Text), f.Mode)
		if err != nil {
			log.Error().Err(err).Interface("File", f).Msg("failed to write file")
		}
		// } else {
		//  	log.Trace().Msg("updating file to match")
		// }
	}

	return nil
}

func (f *File) Exists() bool {
	if _, err := os.Stat(f.Path); err == nil {
		return true
	}
	return false
}

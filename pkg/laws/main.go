// Copyright © 2023 Iggy <iggy@theiggy.com>
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

// Package laws - Laws describe the state of the system
package laws

import (
	"bytes"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/iggy/govern/pkg/facts"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// RetryOpts - retry options
type RetryOpts struct {
	Attempts uint
	Until    bool
	Interval uint
	Splay    uint
}

// CommonFields - common fields for all objects
type CommonFields struct {
	Present  bool
	Uses     []string
	UsedBy   []string
	Needs    []string
	NeededBy []string
	Before   []string
	After    []string
	Reload   bool
	RunAs    string
	Retry    RetryOpts
}

// Laws - describe the state of the system
type Laws struct {
	Users      []User
	Groups     []Group
	Packages   []Package
	Containers []Container
	Scripts    []Script
	Files      []File
	Mounts     []Mount
	Services   []Service
}

// ProcessFile - process a yaml file
func ProcessFile(lawsFile string, pretend bool) error {
	laws := &Laws{}
	log.Trace().Interface("laws-pre", laws).Msg("laws before being parsed")
	log.Debug().Msgf("Processing file: %s", lawsFile)

	// setup templating
	var lawsWr bytes.Buffer
	funcMap := sprig.TxtFuncMap()
	tmpl := template.Must(template.New(filepath.Base(lawsFile)).Funcs(funcMap).ParseFiles(lawsFile))
	log.Trace().Interface("tmpl", tmpl).Msg("what is tmpl?")
	log.Trace().Interface("tmpls", tmpl.Templates()).Msg("what tmpls?")
	eerr := tmpl.Execute(&lawsWr, map[string]interface{}{"facts": facts.Facts}) // TODO pass more stuff to templates
	rendered := lawsWr.Bytes()
	if eerr != nil {
		log.Warn().Err(eerr).Msgf("Failed to execute tmpl: %v", rendered)
		return eerr
	}
	log.Trace().Bytes("rendered", rendered).Msg("")

	err := yaml.Unmarshal(rendered, laws)
	if err != nil {
		log.Warn().Err(err).Msg("Error loading YAML")
		return err
	}

	log.Trace().Interface("laws", laws).Msg("")
	for _, group := range laws.Groups {
		err = group.Ensure(pretend)
		if err != nil {
			log.Warn().Err(err).Msg("could not ensure group")
			return err
		}
	}
	for _, user := range laws.Users {
		err = user.Ensure(pretend)
		if err != nil {
			return err
		}
	}
	for _, pkg := range laws.Packages {
		pkg.Ensure(pretend)
	}
	for _, cntr := range laws.Containers {
		err = cntr.Ensure(pretend)
		if err != nil {
			return err
		}
	}
	for _, file := range laws.Files {
		err = file.Ensure(pretend)
		if err != nil {
			return err
		}
	}
	for _, mount := range laws.Mounts {
		err = mount.Ensure(pretend)
		if err != nil {
			return err
		}
	}
	for _, script := range laws.Scripts {
		err = script.Run(pretend)
		if err != nil {
			return err
		}
	}
	return nil
}

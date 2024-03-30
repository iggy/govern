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

// RetryOpts - retry options
type RetryOpts struct {
	Attempts uint // how many times to try to apply the law
	Until    bool // ??? copied from Salt, probably not necessary
	Interval uint // how long to wait between tries
	Splay    uint // how much variance to add to the interval, useful for thundering herd type scenarios
}

// TODO embedded structs don't work with yaml
// CommonFields - common fields for all objects
// type CommonFields struct {
// 	Name   string
// 	Before []string
// 	After  []string
// 	RunAs  string // user to run as (not implemented)

// 	// other stuff I may do some day
// 	// AfterIf []string // requisites that may not exist due to templating
// 	// Present bool // I think this is supposed to be whether some law is used or not
// 	// Uses     []string // when a law uses some outcome of another law
// 	// UsedBy   []string
// 	// Needs    []string
// 	// NeededBy []string
// 	// Reload   bool     // reload laws/facts after applied, useful to update things like a fact that lists packages installed, services installed, etc
// 	// Retry RetryOpts
// }

// This doesn't do anything, still have to unmarshal all of the common fields in each unmarshaler
// func (c *CommonFields) UnmarshalYAML(value *yaml.Node) error {
// 	// log.Trace().Interface("Node", value).Msg("UnmarshalYAML")
// 	if value.Tag != "!!map" {
// 		return fmt.Errorf("unable to unmarshal yaml: value not map (%s)", value.Tag)
// 	}
// 	c.RunAs = "root"
// 	for i, node := range value.Content {
// 		log.Trace().Interface("node1", node).Msg("commonfields unmarshal")
// 		switch node.Value {
// 		case "name":
// 			c.Name = value.Content[i+1].Value
// 		case "after":
// 			for _, j := range value.Content[i+1].Content {
// 				c.After = append(c.After, j.Value)
// 			}
// 		case "before":
// 			for _, j := range value.Content[i+1].Content {
// 				c.Before = append(c.Before, j.Value)
// 			}
// 		case "runas":
// 			c.RunAs = value.Content[i+1].Value

// 			// case "uid":
// 			// 	u.UID, _ = strconv.ParseUint(value.Content[i+1].Value, 10, 64)
// 		}
// 	}

// 	return nil
// }

// Laws - describe the state of the system
// TODO should really just turn this into a list of `Law`
//
//	type Laws struct {
//		Users        []User
//		Groups       []Group
//		Packages     []Package
//		PackageRepos []PackageRepo
//		Containers   []Container
//		Scripts      []Script
//		Files        FileTemplate
//		Mounts       []Mount
//		Services     []Service
//	}
//
// type Laws2 map[string]interface{}
type Laws3 struct {
	Users struct {
		Present []*User
	}
	Groups struct {
		Present []*Group
	}
	Packages struct {
		Installed []*Package
	}
	PackageRepos struct {
		Present []*PackageRepo
	} `yaml:"package_repos"`
	Containers struct {
		// FIXME revisit this naming
		Running []*Container
	}
	Scripts struct {
		Run []*Script
	}
	Files struct {
		Templates []*FileTemplate
		Inserts   []*FileInsert
		Changes   []*FileChange
		Links     []*FileLink
	}
	Mounts struct {
		Exists []*Mount
		Absent []*AbsentMount
	}
	Services struct {
		Enabled []*Service
	}
	SSH struct {
		AuthorizedKeys []*SSHKey `yaml:"authorized_keys"`
	} `yaml:"ssh"`
}

// type Laws2[T comparable] map[T]struct {
// Laws []Law
// }

// func NewLaws[T comparable]() Laws2[T] {
// 	return make(Laws2[T])
// }

type Law interface {
	// User | Group | Package | Container | Script | FileTemplate | FileInsert | FileChange | Mount | Service

	Ensure(bool) error
}

// ProcessFile - process a yaml file
// func ProcessFile(lawsFile string, pretend bool) error {
// 	laws := &Laws{}
// 	log.Trace().Interface("laws-pre", laws).Msg("laws before being parsed")
// 	log.Debug().Msgf("Processing file: %s", lawsFile)

// 	// setup templating
// 	var lawsWr bytes.Buffer
// 	funcMap := sprig.TxtFuncMap()
// 	tmpl := template.Must(template.New(filepath.Base(lawsFile)).Funcs(funcMap).ParseFiles(lawsFile))
// 	log.Trace().Interface("tmpl", tmpl).Msg("what is tmpl?")
// 	log.Trace().Interface("tmpls", tmpl.Templates()).Msg("what tmpls?")
// 	eerr := tmpl.Execute(&lawsWr, map[string]interface{}{"facts": facts.Facts}) // TODO pass more stuff to templates
// 	rendered := lawsWr.Bytes()
// 	if eerr != nil {
// 		log.Warn().Err(eerr).Msgf("Failed to execute tmpl: %v", rendered)
// 		return eerr
// 	}
// 	log.Trace().Bytes("rendered", rendered).Msg("")

// 	err := yaml.Unmarshal(rendered, laws)
// 	if err != nil {
// 		log.Warn().Err(err).Msg("Error loading YAML")
// 		return err
// 	}

// 	log.Trace().Interface("laws", laws).Msg("")
// 	// TODO these are in a funky order right now to cope with the lack of actual dependencies
// 	for _, group := range laws.Groups {
// 		err = group.Ensure(pretend)
// 		if err != nil {
// 			log.Warn().Err(err).Msg("could not ensure group")
// 			return err
// 		}
// 	}
// 	for _, user := range laws.Users {
// 		err = user.Ensure(pretend)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	for _, pkg := range laws.Packages {
// 		err = pkg.Ensure(pretend)
// 		if err != nil {
// 			return err
// 		}

// 	}
// 	for _, cntr := range laws.Containers {
// 		err = cntr.Ensure(pretend)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	// for _, file := range laws.Files.FileTemplate {
// 	// 	err = file.Ensure(pretend)
// 	// 	if err != nil {
// 	// 		return err
// 	// 	}
// 	// }
// 	for _, mount := range laws.Mounts {
// 		err = mount.Ensure(pretend)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	for _, script := range laws.Scripts {
// 		err = script.Run(pretend)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	for _, service := range laws.Services {
// 		err = service.Ensure(pretend)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

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
type Laws struct {
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
		Absent  []*PackageRepo
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

type Law interface {
	// User | Group | Package | Container | Script | FileTemplate | FileInsert | FileChange | Mount | Service

	Ensure(bool) error
}

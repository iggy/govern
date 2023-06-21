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

package facts

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type DistroFacts struct {
	Name       string
	Slug       string
	Family     string
	Version    string
	Codename   string
	InitSystem string
}

// DistroAlpine - return true if we are on alpine distro
func DistroAlpine() bool {
	ar, err := ioutil.ReadFile("/etc/alpine-release")
	if err != nil {
		return false
	}
	Facts.Distro.Name = "Alpine"
	Facts.Distro.Slug = "alpine"
	Facts.Distro.Family = "alpine"
	Facts.Distro.Version = strings.TrimSpace(string(ar))
	// This could technically be different, but never heard of it in practice
	Facts.Distro.InitSystem = "openrc"
	return true
}

// DistroUbuntu - return true if we are on a Ubuntu distro
func DistroUbuntu() bool {
	lsb, err := os.Open("/etc/lsb-release")

	switch err.(type) {
	case *os.PathError:
		return false
	default:
		scanner := bufio.NewScanner(lsb)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "DISTRIB_RELEASE") {
				Facts.Distro.Version = fmt.Sprint(strings.Split(line, "=")[1])
			}
			if strings.HasPrefix(line, "DISTRIB_CODENAME") {
				Facts.Distro.Codename = fmt.Sprint(strings.Split(line, "=")[1])
			}
		}
		if err = scanner.Err(); err != nil {
			log.Panicln(err)
		}
		Facts.Distro.Name = "Ubuntu"
		Facts.Distro.Slug = "ubuntu"
		Facts.Distro.Family = "debian"
		// This is the one that's been in use for a while
		// but it could technically be upstart as well
		// FIXME actually determine which one is in use
		Facts.Distro.InitSystem = "systemd"
		return true
	}
}

func init() {
	DistroAlpine()
	DistroUbuntu()
	// TODO improve the detection of the init system
	switch Facts.Distro.Family {
	case "alpine":
		Facts.InitSystem = "openrc"
	case "debian":
		Facts.InitSystem = "systemd"
	default:
		// :sad_corgi:
		Facts.InitSystem = "systemd"
	}
}

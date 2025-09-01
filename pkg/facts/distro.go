// Copyright © 2025 Iggy <iggy@theiggy.com>
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
	"log"
	"os"
	"strings"
)

// DistroFacts - holds distro information
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
	ar, err := os.ReadFile("/etc/alpine-release")
	if err != nil {
		return false
	}
	Facts.Distro.Name = "Alpine"
	Facts.Distro.Slug = "alpine"
	Facts.Distro.Family = "alpine"
	Facts.Distro.Version = strings.TrimSpace(string(ar))
	// This could technically be different, but never heard of it in practice
	// Facts.Distro.InitSystem = "openrc"
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
		// Facts.Distro.InitSystem = "systemd"
		return true
	}
}

// DistroArch - return true if we are on an Arch Linux distro
func DistroArch() bool {
	if _, err := os.Stat("/etc/arch-release"); err == nil {
		Facts.Distro.Name = "Arch Linux"
		Facts.Distro.Slug = "arch"
		Facts.Distro.Family = "arch"
		// Arch Linux does not have a version number
		Facts.Distro.Version = "rolling"
		Facts.Distro.InitSystem = "systemd"
		return true
	}
	return false
}

// DistroRHEL - return true if we are on a Red Hat Enterprise Linux distro
func DistroRHEL() bool {
	if _, err := os.Stat("/etc/redhat-release"); err == nil {
		content, err := os.ReadFile("/etc/redhat-release")
		if err != nil {
			return false
		}
		line := strings.TrimSpace(string(content))
		Facts.Distro.Name = "Red Hat Enterprise Linux"
		Facts.Distro.Slug = "rhel"
		Facts.Distro.Family = "rhel"
		// Extract version from the release file
		parts := strings.Fields(line)
		if len(parts) > 6 {
			Facts.Distro.Version = parts[6]
		} else {
			Facts.Distro.Version = "unknown"
		}
		Facts.Distro.InitSystem = "systemd"
		return true
	}
	return false
}

// DistroFedora - return true if we are on a Fedora distro
func DistroFedora() bool {
	if _, err := os.Stat("/etc/fedora-release"); err == nil {
		content, err := os.ReadFile("/etc/fedora-release")
		if err != nil {
			return false
		}
		line := strings.TrimSpace(string(content))
		Facts.Distro.Name = "Fedora"
		Facts.Distro.Slug = "fedora"
		Facts.Distro.Family = "rhel"
		// Extract version from the release file
		parts := strings.Fields(line)
		if len(parts) > 2 {
			Facts.Distro.Version = parts[2]
		} else {
			Facts.Distro.Version = "unknown"
		}
		Facts.Distro.InitSystem = "systemd"
		return true
	}
	return false
}

// determine what init system is in use
func initSystem() string {
	// Check for specific init system executables or files
	if _, err := os.Stat("/sbin/init"); err == nil {
		// Check if /sbin/init is a symlink and resolve it
		initPath, err := os.Readlink("/sbin/init")
		if err == nil {
			switch {
			case strings.Contains(initPath, "systemd"):
				return "systemd"
			case strings.Contains(initPath, "openrc"):
				return "openrc"
			case strings.Contains(initPath, "runit"):
				return "runit"
			case strings.Contains(initPath, "sysvinit"):
				return "sysvinit"
			case strings.Contains(initPath, "upstart"):
				return "upstart"
			}
		}
	}

	// Fallback to checking for specific init system executables
	if _, err := os.Stat("/bin/systemctl"); err == nil {
		return "systemd"
	}
	if _, err := os.Stat("/etc/init.d"); err == nil {
		return "sysvinit"
	}
	if _, err := os.Stat("/etc/runit"); err == nil {
		return "runit"
	}
	if _, err := os.Stat("/etc/openrc"); err == nil {
		return "openrc"
	}
	if _, err := os.Stat("/sbin/initctl"); err == nil {
		return "upstart"
	}

	// Default to systemd if no specific init system is detected
	return "systemd"
}

func init() {
	Facts.InitSystem = initSystem()
	if DistroAlpine() {
		return
	}
	if DistroUbuntu() {
		return
	}
	if DistroArch() {
		return
	}
	if DistroRHEL() {
		return
	}
	if DistroFedora() {
		return
	}
}

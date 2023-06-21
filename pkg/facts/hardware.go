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
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// GetSystemUUID - return/fill in the system UUID
// TODO if we can't find one, generate our own and store it in our cache dir
func GetSystemUUID() string {
	// The system UUID can be in one of a few places
	// /sys/class/dmi/id/product_uuid
	// /sys/class/dmi/id/board_serial
	uuid, err := ioutil.ReadFile("/sys/class/dmi/id/product_uuid")
	if err != nil {
		// log.Printf("error getting System UUID: %v\n", err)
		if os.IsPermission(err) {
			Facts.SystemUUID = fmt.Sprintf("Unable to open UUID file, are you root? (%s)", err)
		} else {
			Facts.SystemUUID = fmt.Sprintf("error: %v", err)
		}
	} else {
		Facts.SystemUUID = strings.TrimSpace(string(uuid))
	}
	return Facts.SystemUUID
}

// CPUInfoFacts - info about the CPU(s) in the system
type CPUInfoFacts struct {
	Arch    string
	Vendor  string
	Model   string
	Cores   int
	Threads int
	Flags   []string
}

// GetCPUInfo - fill in CPUInfo struct
func GetCPUInfo() {
	// arch/machine dod come from uname syscall, but that shit is fragile
	// don't know if this is better, but we'll try
	Facts.CPUInfo.Arch = runtime.GOARCH

	// model/cores/etc comes from /proc/cpuinfo
	ci, err := os.Open("/proc/cpuinfo")

	switch err.(type) {
	case *os.PathError:
		log.Print("failed to open /proc/cpuinfo")
	default:
		scanner := bufio.NewScanner(ci)
		for scanner.Scan() {
			line := scanner.Text()

			// vendor ID for x86/x86_64
			if strings.HasPrefix(line, "vendor_id") {
				vid := fmt.Sprint(strings.TrimSpace(strings.Split(line, ":")[1]))
				if vid == "GenuineIntel" {
					Facts.CPUInfo.Vendor = "intel"
				}
				if vid == "AuthenticAMD" {
					Facts.CPUInfo.Vendor = "amd"
				}

			}
			// https://github.com/util-linux/util-linux/blob/master/sys-utils/lscpu-arm.c
			// vendor ID for arm (ish)
			if strings.HasPrefix(line, "CPU implementor") {
				ci := fmt.Sprint(strings.TrimSpace(strings.Split(line, ":")[1]))
				switch ci {
				case "0x41":
					Facts.CPUInfo.Vendor = "arm"
				case "0x42":
					Facts.CPUInfo.Vendor = "broadcom"
				case "0x43":
					Facts.CPUInfo.Vendor = "cavium"

				}

			}
			if strings.HasPrefix(line, "CPU part") {
				ci := fmt.Sprint(strings.TrimSpace(strings.Split(line, ":")[1]))
				switch ci {
				case "0xd0b":
					Facts.CPUInfo.Model = "Cortex-A76"
				case "0xd05":
					Facts.CPUInfo.Model = "Cortex-A55"
				}
			}

			if strings.HasPrefix(line, "model name") {
				Facts.CPUInfo.Model = fmt.Sprint(strings.TrimSpace(strings.Split(line, ":")[1]))
			}
			if strings.HasPrefix(line, "cpu cores") {
				Facts.CPUInfo.Cores, err = strconv.Atoi(strings.TrimSpace(strings.Split(line, ":")[1]))
				if err != nil {
					log.Printf("couldn't convert cores: %v\n", err)
				}
			}
			if strings.HasPrefix(line, "siblings") {
				Facts.CPUInfo.Threads, err = strconv.Atoi(strings.TrimSpace(strings.Split(line, ":")[1]))
				if err != nil {
					log.Printf("couldn't convert threads: %v\n", err)
				}
			}
			if strings.HasPrefix(line, "flags") {
				Facts.CPUInfo.Flags = strings.Split(strings.Split(line, ":")[1], " ")[1:]
			}
		}
	}
}

func init() {
	GetSystemUUID()
	GetCPUInfo()
}

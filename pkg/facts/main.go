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
	"os"
	"syscall"

	"github.com/rs/zerolog/log"
)

// TODO - other facts to add
//   govern version
//   other versions

type facts struct {
	Hostname    string
	UID         int
	EUID        int
	GID         int
	EGID        int
	Groups      []int
	PID         int
	PPID        int
	Environ     []string
	SystemUUID  string
	MemoryTotal uint64
	CPUInfo     CPUInfoFacts
	Distro      DistroFacts
	Network     NetworkFacts
}

var Facts facts

func init() {
	Facts.Hostname, _ = os.Hostname()
	Facts.UID = os.Getuid()
	Facts.EUID = os.Geteuid()
	Facts.GID = os.Getgid()
	Facts.EGID = os.Getegid()
	Facts.Groups, _ = os.Getgroups()
	Facts.PID = os.Getpid()
	Facts.PPID = os.Getppid()
	Facts.Environ = os.Environ()

	// TODO this struct has other possibly useful stuff (uptime, swap, etc)
	sysinfo := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(sysinfo)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to run syscall.Sysinfo()")
	}
	Facts.MemoryTotal = sysinfo.Totalram
}

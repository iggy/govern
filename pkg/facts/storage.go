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

package facts

import (
	"fmt"

	"github.com/jaypipes/ghw"
	"github.com/rs/zerolog/log"
)

type DiskInfo struct {
	Name       string // (sda, nvme0, vda, etc)
	MountPoint string // where it's mounted
}

type StorageFacts struct {
	LocalDisks []DiskInfo // list of local disks
	// 	RootDisk string // which of the local disks is mounted at /
	// RootFSType string // what format is the rootfs (btrfs, xfs, ext4, bcachefs, etc)
}

func (f *StorageFacts) GetRoot() (*DiskInfo, error) {
	for _, di := range f.LocalDisks {
		if di.MountPoint == "/" {
			return &di, nil
		}
	}
	return nil, fmt.Errorf("unable to find rootfs device")
}

func GetStorageFactsInfo() {
	// Facts.Storage
	block, err := ghw.Block()
	if err != nil {
		log.Error().Err(err).Msg("failed to get block info from ghw")
	}
	for _, disk := range block.Disks {
		for _, part := range disk.Partitions {
			di := &DiskInfo{part.Name, part.MountPoint}
			Facts.Storage.LocalDisks = append(Facts.Storage.LocalDisks, *di)
		}
	}
}

func init() {
	GetStorageFactsInfo()
}

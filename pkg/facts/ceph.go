// Copyright © 2024 Iggy <iggy@theiggy.com>
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
	"encoding/json"
	"os/exec"
	"strconv"

	"github.com/rs/zerolog/log"
)

// CephVolumeTags contains parsed Ceph tags from LVM volumes
type CephVolumeTags struct {
	BlockDevice        string `json:"ceph.block_device"`
	BlockUUID          string `json:"ceph.block_uuid"`
	CephxLockboxSecret string `json:"ceph.cephx_lockbox_secret"`
	ClusterFSID        string `json:"ceph.cluster_fsid"`
	ClusterName        string `json:"ceph.cluster_name"`
	CrushDeviceClass   string `json:"ceph.crush_device_class"`
	DBDevice           string `json:"ceph.db_device"`
	DBUUID             string `json:"ceph.db_uuid"`
	Encrypted          string `json:"ceph.encrypted"`
	OSDFSID            string `json:"ceph.osd_fsid"`
	OSDID              string `json:"ceph.osd_id"`
	OSDSpecAffinity    string `json:"ceph.osdspec_affinity"`
	Type               string `json:"ceph.type"`
	VDO                string `json:"ceph.vdo"`
	WALDevice          string `json:"ceph.wal_device"`
	WALUUID            string `json:"ceph.wal_uuid"`
}

// CephLVMVolume represents a single LVM volume used by Ceph
type CephLVMVolume struct {
	Devices []string       `json:"devices"`
	LVName  string         `json:"lv_name"`
	LVPath  string         `json:"lv_path"`
	LVSize  string         `json:"lv_size"`
	LVTags  string         `json:"lv_tags"`
	LVUUID  string         `json:"lv_uuid"`
	Name    string         `json:"name"`
	Path    string         `json:"path"`
	Tags    CephVolumeTags `json:"tags"`
	Type    string         `json:"type"`
	VGName  string         `json:"vg_name"`
}

// CephOSD represents a Ceph OSD and its associated volumes
type CephOSD struct {
	ID      int
	Volumes []CephLVMVolume
}

// CephFacts contains facts about Ceph storage
type CephFacts struct {
	LVMVolumes map[string][]CephLVMVolume // Map of OSD ID to volumes
	OSDs       []CephOSD                  // Parsed OSD information
}

// GetCephLVMVolumes retrieves Ceph LVM volume information
func GetCephLVMVolumes() (map[string][]CephLVMVolume, error) {
	// Check if ceph-volume command exists
	_, err := exec.LookPath("ceph-volume")
	if err != nil {
		log.Debug().Msg("ceph-volume command not found, skipping Ceph facts")
		return nil, nil
	}

	// Run ceph-volume lvm list --format json
	cmd := exec.Command("ceph-volume", "lvm", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		log.Warn().Err(err).Msg("failed to run ceph-volume lvm list")
		return nil, err
	}

	// Parse JSON output
	var volumes map[string][]CephLVMVolume
	if err := json.Unmarshal(output, &volumes); err != nil {
		log.Error().Err(err).Msg("failed to parse ceph-volume lvm list JSON output")
		return nil, err
	}

	return volumes, nil
}

// ParseOSDs converts the LVM volumes map into a structured list of OSDs
func (cf *CephFacts) ParseOSDs() {
	if cf.LVMVolumes == nil {
		return
	}

	cf.OSDs = make([]CephOSD, 0, len(cf.LVMVolumes))
	for osdIDStr, volumes := range cf.LVMVolumes {
		osdID, err := strconv.Atoi(osdIDStr)
		if err != nil {
			log.Warn().Str("osd_id", osdIDStr).Msg("failed to parse OSD ID")
			continue
		}

		osd := CephOSD{
			ID:      osdID,
			Volumes: volumes,
		}
		cf.OSDs = append(cf.OSDs, osd)
	}
}

// GetBlockVolume returns the block volume for an OSD (if it exists)
func (osd *CephOSD) GetBlockVolume() *CephLVMVolume {
	for i := range osd.Volumes {
		if osd.Volumes[i].Type == "block" {
			return &osd.Volumes[i]
		}
	}
	return nil
}

// GetDBVolume returns the DB volume for an OSD (if it exists)
func (osd *CephOSD) GetDBVolume() *CephLVMVolume {
	for i := range osd.Volumes {
		if osd.Volumes[i].Type == "db" {
			return &osd.Volumes[i]
		}
	}
	return nil
}

// GetWALVolume returns the WAL volume for an OSD (if it exists)
func (osd *CephOSD) GetWALVolume() *CephLVMVolume {
	for i := range osd.Volumes {
		if osd.Volumes[i].Type == "wal" {
			return &osd.Volumes[i]
		}
	}
	return nil
}

func init() {
	volumes, err := GetCephLVMVolumes()
	if err != nil {
		log.Debug().Err(err).Msg("failed to get Ceph LVM volumes")
		return
	}

	Facts.Ceph.LVMVolumes = volumes
	Facts.Ceph.ParseOSDs()
}

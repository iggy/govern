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

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iggy/govern/pkg/facts"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// factsCmd represents the facts command
var factsCmd = &cobra.Command{
	Use:   "facts",
	Short: "Show facts about the system",
	Long: `This will show you the facts about the current system.

Use this to see the facts you can reference in the laws yaml templates.
`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Trace().Msg("facts called")

		w := tabwriter.NewWriter(os.Stdout, 1, 4, 4, ' ', 0)
		fmt.Fprintf(w, "distro name:\t%v\n", facts.Facts.Distro.Name)
		fmt.Fprintf(w, "distro slug:\t%v\n", facts.Facts.Distro.Slug)
		fmt.Fprintf(w, "distro family:\t%v\n", facts.Facts.Distro.Family)
		fmt.Fprintf(w, "distro version:\t%v\n", facts.Facts.Distro.Version)
		fmt.Fprintf(w, "distro codename:\t%v\n", facts.Facts.Distro.Codename)
		fmt.Fprintf(w, "System UUID:\t%v\n", facts.Facts.SystemUUID)
		fmt.Fprintf(w, "Total Memory:\t%d (%dGiB)\n\n", facts.Facts.MemoryTotal, facts.Facts.MemoryTotal>>30)
		w.Flush()

		fmt.Fprintf(w, "CPUInfo:\t\n")
		fmt.Fprintf(w, "\tArch:\t%v\n", facts.Facts.CPUInfo.Arch)
		fmt.Fprintf(w, "\tVendor:\t%v\n", facts.Facts.CPUInfo.Vendor)
		fmt.Fprintf(w, "\tModel:\t%v\n", facts.Facts.CPUInfo.Model)
		fmt.Fprintf(w, "\tCores:\t%v\n", facts.Facts.CPUInfo.Cores)
		fmt.Fprintf(w, "\tThreads:\t%v\n\n", facts.Facts.CPUInfo.Threads)
		// fmt.Fprintf(w, "\tFlags:\t%#v\n", facts.Facts.CPUInfo.Flags)

		// fmt.Fprintf(w, "net interfaces:\t%#v\n", facts.Facts.Network.Interfaces)
		fmt.Fprintf(w, "net interfaces:\n")
		for _, iface := range facts.Facts.Network.Interfaces {
			fmt.Fprintf(w, "\t%s\t%s\n", iface.Name, iface.HardwareAddr)
		}
		w.Flush()

		// fmt.Fprintf(w, "Env:\t %v\n", facts.Facts.Environ)
		w.Flush()
		// fmt.Println(facts.Facts)
	},
}

func init() {
	localCmd.AddCommand(factsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// factsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// factsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

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
	"path/filepath"

	"github.com/iggy/govern/pkg/laws"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "apply changes to a system from a local config",
	Long: `Apply changes to a system using laws yaml templates from a
local directory.
`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info().Msg("apply called")
		file, _ := cmd.Flags().GetString("file")
		directory, _ := cmd.Flags().GetString("directory")

		if file != "" {
			err := laws.ProcessFile(file, false)
			if err != nil {
				log.Fatal().Msgf("failed to process file (%s): %v\n", file, err)
			}
		}
		if directory != "" {
			files, err := filepath.Glob(filepath.Join(directory, "*"))
			if err != nil {
				log.Fatal().Msgf("Invalid pattern")
			}
			for _, file := range files {
				err := laws.ProcessFile(file, false)
				if err != nil {
					log.Fatal().Msgf("failed to process file (%s): %v\n", file, err)
				}
			}
		}
	},
}

func init() {
	localCmd.AddCommand(applyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// applyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// applyCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	applyCmd.Flags().StringP("file", "f", "", "local Laws yaml file")
	applyCmd.Flags().StringP("directory", "d", "", "directory with Laws yaml files")
}

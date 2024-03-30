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
	"github.com/iggy/govern/pkg/laws"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// pretendCmd represents the pretend command
var pretendCmd = &cobra.Command{
	Use:   "pretend",
	Short: "pretend an apply",
	Long:  `Output what changes an apply would run using the given configs.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug().Msg("pretend called")
		// file, _ := cmd.Flags().GetString("file")
		// directory, _ := cmd.Flags().GetString("directory")

		file, _ := cmd.Flags().GetString("file")
		directory, _ := cmd.Flags().GetString("directory")
		var toParse string

		if file != "" {
			toParse = file
		}
		if directory != "" {
			toParse = directory
		}
		sorted, err := laws.ParseFiles(toParse)
		if err != nil {
			log.Fatal().Msgf("lint: failed to process (%s): %v\n", toParse, err)
		}
		for _, v := range sorted {
			// vValues := reflect.ValueOf(v.Label().Law)
			// vTypes := vValues.Type()

			// for i := 0; i < vValues.Elem().NumField(); i++ {
			// 	log.Debug().
			// 		Interface("vValues i", vValues.Field(i)).
			// 		Msgf("vValues i: %v - %v", vValues.Field(i), vValues.Field(i).Type())
			// }

			l := v.Label().Law
			err = l.Ensure(true)
			if err != nil {
				// we don't need to fatal on a pretend
				log.Error().
					Err(err).
					Str("name", v.Label().Name).
					Str("type", v.Label().Type).
					Msg("failed to ensure")
			}

			// log.Trace().
			// 	Interface("v", v).
			// 	Interface("vValues", vValues).
			// 	Interface("vTypes", vTypes.Elem().Field(0)).
			// 	Interface("type", reflect.TypeOf(v.Label().Law)).
			// 	Msgf("90: %v - %v - %v",
			// 		v,
			// 		reflect.TypeOf(reflect.ValueOf(v.Label().Law).Interface()).String(),
			// 		reflect.ValueOf(v.Label().Law).Interface(),
			// 	)
			// switch lt := reflect.ValueOf(v.Label().Law).Interface().(type) {
			// case laws.User:
			// 	log.Debug().Interface("lt", lt).Msgf("%v", lt)
			// default:
			// 	log.Debug().Interface("lt", lt).Msgf("default %v", lt)

			// }
		}
		// 		log.Debug().Msgf("distro slug: %s\n", facts.Facts.Distro.Slug)
		// log.Debug().Msgf("hostname: %v\n", facts.Facts.Hostname)

		// if file != "" {
		// 	err := laws.ProcessFile(file, true)
		// 	if err != nil {
		// 		log.Fatal().Msgf("pretend: failed to process file (%s): %v\n", file, err)
		// 	}
		// }
		// if directory != "" {
		// 	files, err := filepath.Glob(filepath.Join(directory, "*"))
		// 	if err != nil {
		// 		log.Fatal().Msgf("pretend: Invalid pattern")
		// 	}
		// 	for _, file := range files {
		// 		err := laws.ProcessFile(file, true)
		// 		if err != nil {
		// 			log.Fatal().Msgf("pretend: failed to process file (%s): %v\n", file, err)
		// 		}
		// 	}
		// }
	},
}

func init() {
	localCmd.AddCommand(pretendCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// pretendCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// pretendCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	pretendCmd.Flags().StringP("file", "f", "", "local state file")
	pretendCmd.Flags().StringP("directory", "d", "", "directory with Laws yaml files")
}

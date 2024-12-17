// Copyright © 2023 Iggy <iggy@theiggy.com>
//
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

// TODO
// * file::patch
// * file::symlink/hardlink
// * file::touch

package laws

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// // File represents a file in the filesystem
//
//	type File struct {
//		Templates []Template
//		Inserts   []Insert
//		Changes   []Change
//	}

type fileCommon struct {
	Name    string `yaml:"name"`
	Before  []string
	After   []string
	MakeDir bool        `yaml:"make_dir"` // make the parent dir
	User    string      // user/uid owner of the file
	Group   string      // group/gid owner of the file
	Mode    fs.FileMode // file mode TODO maybe default to 400?
	Backup  bool        // whether to backup the file before changing

}

type FileTemplate struct {
	fileCommon `yaml:",inline"`
	// Name         string      // file path
	// MakeDir      bool        `yaml:"make_dir"` // make the parent dir
	// User         string      // user/uid owner of the file
	// Group        string      // group/gid owner of the file
	// Mode         fs.FileMode // file mode TODO maybe default to 400?
	Text         string // text template
	TemplatePath string // path to a file to use instead of Text (unimpl)
	// Backup       bool        // whether to backup the file before changing

	// CommonFields
	// Name   string
	// Before []string
	// After  []string
}
type FileInsert struct {
	fileCommon `yaml:",inline"`
	// Name       string
	// MakeDir    bool `yaml:"make_dir"` // make the parent dir
	AfterLine  string
	BeforeLine string
	LineNum    int64
	Text       string

	// CommonFields
	// Name   string
	// Before []string
	// After  []string
}

type FileChange struct {
	fileCommon `yaml:",inline"`
	Search     string   `yaml:"search"`            // line to search for
	Replace    string   `yaml:"replace,omitempty"` // line to replace with
	Done       string   `yaml:"done"`
	If         []string // should probably convert this into some template logic

	// Name    string `yaml:"name"`
	// MakeDir bool     `yaml:"make_dir"`          // make the parent dir
	// CommonFields
	// Name   string
	// Before []string
	// After  []string
}

type FileLink struct {
	fileCommon `yaml:",inline"`
	Target     string `yaml:"target"` // the target of the link
	Symbolic   bool   `yaml:"symbolic"`
}

// FileOwner
//
// func (f *File) UnmarshalYAML(value *yaml.Node) error {
//     log.Trace().Msg("file unmarshall yaml")
//     return nil
// }

func (f *FileTemplate) UnmarshalYAML(value *yaml.Node) error {
	// f.LineNum = -1

	log.Trace().Interface("Node", value).Msg("UnmarshalYAML fileinsert")
	if value.Tag != "!!map" {
		return fmt.Errorf("unable to unmarshal yaml: value not map (%s)", value.Tag)
	}

	for i, node := range value.Content {
		log.Trace().Interface("node1", node).Msg("")
		switch node.Value {
		// common fields
		case "name":
			f.Name = value.Content[i+1].Value
		case "before":
			for _, j := range value.Content[i+1].Content {
				f.Before = append(f.Before, j.Value)
			}
		case "after":
			for _, j := range value.Content[i+1].Content {
				f.After = append(f.After, j.Value)
			}
		case "make_dir":
			b, err := strconv.ParseBool(value.Content[i+1].Value)
			if err != nil {
				log.Warn().Str("key", node.Value).Str("val", value.Content[i+1].Value).Msg("failed to parse make_dir")
			}
			f.MakeDir = b
		case "user":
			f.User = value.Content[i+1].Value
		case "group":
			f.Group = value.Content[i+1].Value
		case "mode":
			// 0644 (etc) is octal
			fm, err := strconv.ParseInt(value.Content[i+1].Value, 8, 32)
			if err != nil {
				log.Error().Err(err).Msg("failed to parse mode")
			}
			f.Mode = os.FileMode(fm)
			// f.Mode, _ = int(value.Content[i+1].Value)
		case "backup":
			b, err := strconv.ParseBool(value.Content[i+1].Value)
			if err != nil {
				log.Warn().Str("key", node.Value).Str("val", value.Content[i+1].Value).Msg("failed to parse backup")
			}
			f.Backup = b
		// case "after_line":
		// 	f.AfterLine = value.Content[i+1].Value
		// case "before_line":
		// 	f.BeforeLine = value.Content[i+1].Value
		// case "line_num":
		// 	f.LineNum, _ = strconv.ParseInt(value.Content[i+1].Value, 10, 64)
		case "text":
			f.Text = value.Content[i+1].Value

			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
		}
	}
	log.Trace().Interface("f", f).Msg("what's in the box?!?!")

	return nil

}

// Ensure ensures that the file exists with the correct contents
func (f *FileTemplate) Ensure(pretend bool) error {
	log.Trace().Interface("File", f).Msg("file ensure")

	if f.Name == "" {
		return fmt.Errorf("file template name not set")
	}

	if pretend {
		if f.Exists() {
			log.Info().Str("file", f.Name).Msg("file exists, skipping")
		} else {
			log.Info().Str("file", f.Name).Msg("file doesn't exist, would create")
		}
	} else {
		// TODO make sure we aren't overwriting an existing file
		if f.Backup && f.Exists() {
			log.Trace().Msg("backing up file before writing")
			err := os.Rename(f.Name, f.Name+".bak")
			if err != nil {
				log.Error().Err(err).Interface("file", f).Msg("failed to backup file")
			}
		}
		var isDir bool
		fi, err := os.Stat(path.Dir(f.Name))
		if err != nil {
			// this means the path didn't exist, so it's definitely not a dir
			// not exactly an error, we expect this to be the case sometimes
			log.Debug().
				Err(err).
				Str("file", f.Name).
				Interface("fi", fi).
				Msg("failed to stat dir")
			isDir = false
		} else {
			isDir = fi.IsDir()
		}
		if f.MakeDir && !isDir {
			err := os.MkdirAll(path.Dir(f.Name), 0755)
			if err != nil {
				log.Error().
					Err(err).
					Str("file", f.Name).
					Msg("failed to mkdirall for file")
			}
		}
		if !f.Exists() {
			// we just always write the file, opening -< checking -> possibly writing is often slower than just writing
			err := os.WriteFile(f.Name, []byte(f.Text), f.Mode)
			if err != nil {
				log.Error().Err(err).Interface("File", f).Msg("failed to write file")
			}
		} else {
			log.Trace().Msg("updating file to match")
		}
		// ->checking -> possibly writing is often slower than just writing
		err = os.WriteFile(f.Name, []byte(f.Text), f.Mode)
		if err != nil {
			log.Error().Err(err).Interface("File", f).Msg("failed to write file")
		}
		// } else {
		//  	log.Trace().Msg("updating file to match")
		// }
		err = os.Chmod(f.Name, f.Mode)
		if err != nil {
			log.Error().Err(err).Msg("failed to chmod")
		}
	}

	return nil
}

// Exists checks if the file exists
func (f *FileTemplate) Exists() bool {
	if _, err := os.Stat(f.Name); err == nil {
		return true
	}
	return false
}

func (f *FileInsert) UnmarshalYAML(value *yaml.Node) error {
	f.LineNum = -1

	log.Trace().Interface("Node", value).Msg("UnmarshalYAML fileinsert")
	if value.Tag != "!!map" {
		return fmt.Errorf("unable to unmarshal yaml: value not map (%s)", value.Tag)
	}

	for i, node := range value.Content {
		log.Trace().Interface("node1", node).Msg("")
		switch node.Value {
		// common fields
		case "name":
			f.Name = value.Content[i+1].Value
		case "before":
			for _, j := range value.Content[i+1].Content {
				f.Before = append(f.Before, j.Value)
			}
		case "after":
			for _, j := range value.Content[i+1].Content {
				f.After = append(f.After, j.Value)
			}
		case "make_dir":
			b, err := strconv.ParseBool(value.Content[i+1].Value)
			if err != nil {
				log.Warn().Str("key", node.Value).Str("val", value.Content[i+1].Value).Msg("failed to parse make_dir")
			}
			f.MakeDir = b
		case "user":
			f.User = value.Content[i+1].Value
		case "group":
			f.Group = value.Content[i+1].Value
		case "mode":
			// TODO
			// f.Mode, _ = strconv.ParseInt(value.Content[i+1].Value, 10, 32)
		case "backup":
			b, err := strconv.ParseBool(value.Content[i+1].Value)
			if err != nil {
				log.Warn().Str("key", node.Value).Str("val", value.Content[i+1].Value).Msg("failed to parse backup")
			}
			f.Backup = b
		case "after_line":
			f.AfterLine = value.Content[i+1].Value
		case "before_line":
			f.BeforeLine = value.Content[i+1].Value
		case "line_num":
			f.LineNum, _ = strconv.ParseInt(value.Content[i+1].Value, 10, 64)
		case "text":
			f.Text = value.Content[i+1].Value

			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
		}
	}
	log.Trace().Interface("f", f).Msg("what's in the box?!?!")

	return nil

}

func (f *FileInsert) Ensure(pretend bool) error {
	fl := log.With().Str("file insert", f.Name).Logger()

	fl.Debug().Interface("fileinsert", f).Msg("")
	if pretend {
		fl.Info().Msg("would change file")
		return nil
	}
	if f.AfterLine == "" && f.LineNum == -1 {
		fl.Warn().Msg("file insert: no after_line or line_num specified")
		return fmt.Errorf("file insert: no after_line or line_num specified")
	}

	if f.LineNum != -1 {
		fp, err := os.Open(f.Name)
		if err != nil {
			fl.Error().Err(err).Str("file", f.Name).Msg("failed to open file for scanning")
			return err
		}
		defer fp.Close()

		scanner := bufio.NewScanner(fp)
		var newContent []string
		lineN := int64(1)
		for scanner.Scan() {
			line := scanner.Text()
			// fl.Info().Str("line", line).Msg("")
			if lineN == f.LineNum && line == f.Text {
				// replacement line already exists in file
				fl.Debug().
					Str("name", f.Name).
					Int64("line_num", f.LineNum).
					Str("text", f.Text).
					Msg("already done")
				return nil
			}
			if lineN == f.LineNum {
				// we found our line num, so we add the text to the output and then the existing line
				newContent = append(newContent, strings.TrimRight(f.Text, "\r\n"))
				newContent = append(newContent, strings.TrimRight(line, "\r\n"))
			} else {
				newContent = append(newContent, strings.TrimRight(line, "\r\n"))
			}
			// fl.Info().Strs("newContent", newContent).Msg("")
			lineN++
		}
		fp.Close()

		fw, err := os.OpenFile(f.Name, os.O_WRONLY|os.O_TRUNC, f.Mode)
		if err != nil {
			fl.Error().Err(err).Str("file", f.Name).Msg("failed to seek to start of file")
		}
		defer fw.Close()
		_, err = fw.Write([]byte(strings.Join(newContent, "\n")))
		if err != nil {
			fl.Error().Err(err).Str("file", f.Name).Msg("failed to write newContent to file")
		}
		_, err = fw.WriteString("\n")
		if err != nil {
			fl.Error().Err(err).Str("file", f.Name).Msg("failed to write newline to file")
		}

	}
	if f.AfterLine != "" {
		fp, err := os.Open(f.Name)
		if err != nil {
			fl.Error().Err(err).Str("file", f.Name).Msg("failed to open file for scanning")
			return err
		}
		defer fp.Close()

		scanner := bufio.NewScanner(fp)
		var newContent []string
		for scanner.Scan() {
			line := scanner.Text()
			// fl.Info().Str("line", line).Msg("")
			if line == f.Text {
				// replacement line already exists in file
				fl.Debug().
					Str("name", f.Name).
					Str("after_line", f.AfterLine).
					Str("text", f.Text).
					Msg("already done")
				return nil
			}
			// if err != nil {
			// 	fl.Error().Err(err).Msg("failed to match")
			// }
			if line == f.AfterLine {
				// we found our line, so we add it and the text to the output
				newContent = append(newContent, strings.TrimRight(line, "\r\n"))
				newContent = append(newContent, strings.TrimRight(f.Text, "\r\n"))
			} else {
				newContent = append(newContent, strings.TrimRight(line, "\r\n"))
			}
			// fl.Info().Strs("newContent", newContent).Msg("")
		}
		fp.Close()

		fw, err := os.OpenFile(f.Name, os.O_WRONLY|os.O_TRUNC, f.Mode)
		if err != nil {
			fl.Error().Err(err).Str("file", f.Name).Msg("failed to seek to start of file")
		}
		defer fw.Close()
		_, err = fw.Write([]byte(strings.Join(newContent, "\n")))
		if err != nil {
			fl.Error().Err(err).Str("file", f.Name).Msg("failed to write newContent to file")
		}
		_, err = fw.WriteString("\n")
		if err != nil {
			fl.Error().Err(err).Str("file", f.Name).Msg("failed to write newline to file")
		}

	}

	return nil
}

func (f *FileChange) UnmarshalYAML(value *yaml.Node) error {
	// f.LineNum = -1
	// we set the default to a random string that should never appear in a file
	f.Done = "8df59722fca35a8de040c0490e7add0cab6b0751a4c0dc15a066ae174b63f274"

	log.Trace().Interface("Node", value).Msg("UnmarshalYAML fileinsert")
	if value.Tag != "!!map" {
		return fmt.Errorf("unable to unmarshal yaml: value not map (%s)", value.Tag)
	}

	for i, node := range value.Content {
		log.Trace().Interface("node1", node).Msg("")
		switch node.Value {
		// common fields
		case "name":
			f.Name = value.Content[i+1].Value
		case "before":
			for _, j := range value.Content[i+1].Content {
				f.Before = append(f.Before, j.Value)
			}
		case "after":
			for _, j := range value.Content[i+1].Content {
				f.After = append(f.After, j.Value)
			}
		case "make_dir":
			b, err := strconv.ParseBool(value.Content[i+1].Value)
			if err != nil {
				log.Warn().Str("key", node.Value).Str("val", value.Content[i+1].Value).Msg("failed to parse make_dir")
			}
			f.MakeDir = b
		case "user":
			f.User = value.Content[i+1].Value
		case "group":
			f.Group = value.Content[i+1].Value
		case "mode":
			// TODO
			// f.Mode, _ = strconv.ParseInt(value.Content[i+1].Value, 10, 32)
		case "backup":
			b, err := strconv.ParseBool(value.Content[i+1].Value)
			if err != nil {
				log.Warn().Str("key", node.Value).Str("val", value.Content[i+1].Value).Msg("failed to parse backup")
			}
			f.Backup = b
		case "search":
			f.Search = value.Content[i+1].Value
		case "replace":
			f.Replace = value.Content[i+1].Value
		case "done":
			f.Done = value.Content[i+1].Value

			// case "line_num":
			// 	f.LineNum, _ = strconv.ParseInt(value.Content[i+1].Value, 10, 64)
			// case "text":
			// 	f.Text = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
			// case "":
			// m. = value.Content[i+1].Value
		}
	}
	log.Trace().Interface("f", f).Msg("what's in the box?!?!")

	return nil

}

// TODO handle \r's
func (f *FileChange) Ensure(pretend bool) error {
	fl := log.With().Str("file change", f.Name).Logger() // function logger adds some extra info

	fl.Debug().Interface("filechange", f).Msg("")
	if pretend {
		fl.Info().Msg("would change file")
		return nil
	}
	if f.Search == "" && f.Replace == "" {
		fl.Warn().Str("name", f.Name).Msg("failed to ensure filechange, search and replace not set")
		return nil
	}
	fp, err := os.Open(f.Name)
	if err != nil {
		fl.Error().Err(err).Str("file", f.Name).Msg("failed to open file for scanning")
		return err
	}
	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	var newContent []string
	for scanner.Scan() {
		line := scanner.Text()
		// fl.Info().Str("line", line).Msg("")

		// if we already have the replacement line or the done text, we don't need to keep going
		if line == f.Replace || strings.Contains(line, f.Done) {
			// replacement line already exists in file
			fl.Debug().
				Str("name", f.Name).
				Str("search", f.Search).
				Str("replace", f.Replace).
				Msg("already done")
			return nil
		}
		match, err := regexp.MatchString(f.Search, line)
		if err != nil {
			fl.Error().Err(err).Msg("failed to match")
		}
		if match {
			rgx := regexp.MustCompile(f.Search)
			repl := rgx.ReplaceAllString(line, f.Replace)
			newContent = append(newContent, strings.TrimRight(repl, "\r\n"))
		} else {
			newContent = append(newContent, strings.TrimRight(line, "\r\n"))
		}
		// fl.Info().Strs("newContent", newContent).Msg("")
	}
	fp.Close()

	fw, err := os.OpenFile(f.Name, os.O_WRONLY|os.O_TRUNC, f.Mode)
	if err != nil {
		fl.Error().Err(err).Str("file", f.Name).Msg("failed to seek to start of file")
	}
	defer fw.Close()
	_, err = fw.Write([]byte(strings.Join(newContent, "\n")))
	if err != nil {
		fl.Error().Err(err).Str("file", f.Name).Msg("failed to write newContent to file")
	}
	_, err = fw.WriteString("\n")
	if err != nil {
		fl.Error().Err(err).Str("file", f.Name).Msg("failed to write newline to file")
	}

	return nil
}

func (f *FileLink) UnmarshalYAML(value *yaml.Node) error {
	f.Symbolic = true
	type rawFileLink FileLink
	err := value.Decode((*rawFileLink)(f)) // this goes into an infinite loop
	if err != nil && err != io.EOF {
		log.Error().Err(err).Msg("failed to decode yaml")
		return err
	}
	return nil
}

func (f *FileLink) Ensure(pretend bool) error {
	fl := log.With().Str("file link", f.Name).Logger() // function logger adds some extra info
	if pretend {
		fl.Info().Str("target", f.Target).Msg("would link")
	} else {
		// TODO check if target exists and is a symlink
		err := os.Symlink(f.Target, f.Name)
		if err != nil {
			fl.Error().Err(err).Str("target", f.Target).Msg("failed to symlink")
		}
	}
	return nil
}

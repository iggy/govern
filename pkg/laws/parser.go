package laws

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"dario.cat/mergo"
	"github.com/Masterminds/sprig/v3"
	"github.com/goccy/go-yaml"
	"github.com/hmdsefi/gograph"
	"github.com/iggy/govern/pkg/facts"
	"github.com/rs/zerolog/log"
)

// dep graph node that represents each law parsed from the laws yaml files
// i.e. each one represents a user, group, file, etc
type LawNode struct {
	Law   Law
	Group string
	Type  string
	Name  string
}

type Root struct {
	Name string
}

// Ensure - just to fulfill the interface
func (r *Root) Ensure(bool) error {
	return nil
}

// var graph gograph.Graph[*LawNode]

// ParseFiles - parse a file or directory of yaml files to get the laws
// This is a total pain... either I screw myself on the logic by making
// everything a struct or I screw myself on the parsing by using maps and
// interfaces
func ParseFiles(path string) ([]*gograph.Vertex[*LawNode], error) {
	log.Trace().Str("path", path).Msg("parsing files")

	laws := &Laws{}

	graph := gograph.New[*LawNode](gograph.Acyclic())
	rootVertex := gograph.NewVertex[*LawNode](&LawNode{&Root{Name: "root"}, "root", "root", "root"})
	log.Debug().Interface("rootv", rootVertex).Msg("I'm tired of having to constantly (un)comment this")

	// TODO handle single files
	fileSystem := os.DirFS(path)

	// Load shared variables from _variables.yaml if present, and pass into template context
	sharedVars := map[string]interface{}{}
	varsPath := filepath.Join(path, "_variables.yaml")
	if varsData, err := os.ReadFile(varsPath); err == nil {
		if err := yaml.Unmarshal(varsData, &sharedVars); err != nil {
			log.Error().Err(err).Str("path", varsPath).Msg("failed to parse _variables.yaml")
		} else {
			log.Debug().Interface("vars", sharedVars).Msg("loaded shared variables")
		}
	} else {
		log.Debug().Str("path", varsPath).Msg("no _variables.yaml found, skipping shared vars")
	}

	err := fs.WalkDir(fileSystem,
		".",
		func(walkpath string, d fs.DirEntry, walkErr error) error {
			log.Debug().Interface("d", d).Str("path", walkpath).Msg("processing")
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(walkpath) != ".yaml" && filepath.Ext(walkpath) != ".yml" {
				return nil
			}
			if filepath.Base(walkpath) == "_variables.yaml" {
				return nil
			}

			loopLaws := &Laws{}

			fi, err := d.Info()
			if err != nil {
				log.Error().Err(err).Str("path", walkpath).Msg("failed to get info for direntry")
			}
			log.Debug().Interface("fileinfo", fi).Interface("sys", fi.Sys()).Msg("")
			lawsFilePath := filepath.Join(path, walkpath)

			// setup templating
			var lawsWr bytes.Buffer
			funcMap := sprig.GenericFuncMap()
			// this is kind of weird, but you can't have / in the template name
			tmpl := template.Must(
				template.New(filepath.Base(walkpath)).
					Funcs(funcMap).
					ParseFiles(lawsFilePath),
			)
			log.Trace().Interface("tmpl", tmpl).Msg("what is tmpl?")
			log.Trace().Interface("tmpls", tmpl.Templates()).Msg("what tmpls?")
			err = tmpl.Execute(&lawsWr, map[string]interface{}{"facts": facts.Facts, "vars": sharedVars})
			rendered := lawsWr.Bytes()
			if err != nil {
				log.Error().Err(err).Bytes("rendered", rendered).Msg("failed to execute tmpl")
				return err
			}
			log.Trace().Bytes("rendered", rendered).Msg("")

			err = yaml.Unmarshal(rendered, loopLaws)
			if err != nil {
				log.Warn().Err(err).Str("file", walkpath).Msg("Error loading YAML")
				return err
			}

			// Resolve template_path fields into Text so Ensure() can write them
			// without needing access to the laws directory or shared vars.
			for _, ft := range loopLaws.Files.Templates {
				if ft.TemplatePath == "" {
					continue
				}
				if ft.Text != "" {
					return fmt.Errorf("file template %q: both 'text' and 'template_path' are set; use one or the other", ft.Name)
				}
				tmplFilePath := ft.TemplatePath
				if !filepath.IsAbs(tmplFilePath) {
					tmplFilePath = filepath.Join(path, tmplFilePath)
				}
				tmplContent, err := os.ReadFile(tmplFilePath)
				if err != nil {
					log.Error().Err(err).Str("template_path", ft.TemplatePath).Msg("failed to read template_path file")
					return err
				}
				var tmplBuf bytes.Buffer
				tmplFuncMap := sprig.GenericFuncMap()
				t, err := template.New(filepath.Base(tmplFilePath)).Funcs(tmplFuncMap).Parse(string(tmplContent))
				if err != nil {
					log.Error().Err(err).Str("template_path", ft.TemplatePath).Msg("failed to parse template_path file")
					return err
				}
				if err = t.Execute(&tmplBuf, map[string]interface{}{"facts": facts.Facts, "vars": sharedVars}); err != nil {
					log.Error().Err(err).Str("template_path", ft.TemplatePath).Msg("failed to render template_path file")
					return err
				}
				ft.Text = tmplBuf.String()
				log.Debug().
					Str("name", ft.Name).
					Str("template_path", ft.TemplatePath).
					Msg("resolved template_path into text")
			}

			log.Debug().Interface("loopLaws", loopLaws).Msg("")
			err = mergo.Merge(laws, loopLaws, mergo.WithAppendSlice)
			if err != nil {
				log.Error().Err(err).Msg("failed to mergo")
			}

			log.Trace().Interface("laws", laws).Msg("")

			return nil
		},
	)
	if err != nil {
		log.Error().Msg("")
	}

	l1Values := reflect.ValueOf(*laws)
	l1Types := l1Values.Type()
	log.Debug().
		Interface("l1values", l1Values.Type().Name()).
		Interface("l1Types", l1Types.Name()).
		Msg("l1")

	// add all the nodes to the graph first
	// this loop is over users/groups/pkgs/etc structs
	for i := 0; i < l1Values.NumField(); i++ {
		lawsGroup := l1Types.Field(i).Name // users/groups/pkgs/etc
		l2Values := reflect.ValueOf(l1Values.Field(i).Interface())
		l2Types := l2Values.Type()

		// this loop is over present/installed/running/etc
		for j := 0; j < l2Values.NumField(); j++ {
			lawsType := l2Types.Field(j).Name
			l3Values := reflect.ValueOf(l2Values.Field(j).Interface())

			// this loop is over the array of user/group/filetemplate/etc
			for k := 0; k < l3Values.Len(); k++ {
				// for _, k := range l3Values.Slice(0, l3Values.Len()) {
				m := l3Values.Index(k)
				// log.Info().Interface("m", m).Msgf("k loop: %v", m)
				lawsName := m.Elem().FieldByName("Name").String()
				// before := m.FieldByName("Before")
				// after := m.FieldByName("After")
				vGroup := strings.ToLower(lawsGroup)
				vType := strings.ToLower(lawsType)
				vName := strings.ToLower(lawsName)
				log.Trace().
					Str("vGroup", vGroup).
					Str("vType", vType).
					Str("vName", vName).
					Msg("load graph loop")

				// Set State field for PackageRepo based on lawsType (Present/Absent)
				if vGroup == "packagerepos" {
					stateField := m.Elem().FieldByName("State")
					if stateField.IsValid() && stateField.CanSet() {
						stateField.SetString(vType)
					}
				}

				// vtx := gograph.NewVertex[*LawNode](&LawNode{m, lawGroup, lawGroupSetting})
				vtx := gograph.NewVertex[*LawNode](
					&LawNode{
						Law:   m.Interface().(Law),
						Group: vGroup,
						Type:  vType,
						Name:  vName,
					},
				)
				log.Debug().
					Str("type", vtx.Label().Type).
					Str("name", vtx.Label().Name).
					Msgf("l2 vtx: %v", vtx)
				_, err := graph.AddEdge(rootVertex, vtx)
				if err != nil {
					log.Error().Err(err).
						Str("law name", vName).
						Str("law type", vType).
						Str("law group", vGroup).
						Msg("failed to add edge to root")
				}
			}
		}
	}

	// now setup the deps properly
	// this loop is over users/groups/pkgs/etc structs
	for i := 0; i < l1Values.NumField(); i++ {
		lawGroup := l1Types.Field(i).Name // users/groups/pkgs/etc
		l2Values := reflect.ValueOf(l1Values.Field(i).Interface())
		l2Types := l2Values.Type()
		log.Debug().
			Str("lg", lawGroup).
			Interface("value", l1Values.Field(i)).
			Interface("l2values", l2Values).
			Interface("l2Types", l2Types).
			Msgf("l1 kv: %v - %v", l1Values.Field(i).Interface(), l2Types)
		// this loop is over present/installed/running/etc
		for j := 0; j < l2Values.NumField(); j++ {
			lawsType := l2Types.Field(j).Name
			lawGroupSetting := l2Types.Field(j).Name
			l3Values := reflect.ValueOf(l2Values.Field(j).Interface())
			l3Types := l3Values.Type()
			log.Debug().
				Str("name", l2Types.Field(j).Name).
				Str("lg", lawGroup).
				Interface("lgs", lawGroupSetting).
				Interface("value", l2Values.Field(j).Interface()).
				Msgf("l2 kv: v: %v - t: %v", l3Values, l3Types)
			log.Debug().Msgf("l3: %v", l3Values.Slice(0, l3Values.Len()))

			// this loop is over the array of user/group/filetemplate/etc
			for k := 0; k < l3Values.Len(); k++ {
				m := l3Values.Index(k)
				before := m.Elem().FieldByName("Before")
				after := m.Elem().FieldByName("After")
				vGroup := strings.ToLower(lawGroup)
				vType := strings.ToLower(lawsType)
				vName := strings.ToLower(m.Elem().FieldByName("Name").String())
				log.Debug().Msgf("l4a: %v - %v", m, m.Type())
				log.Debug().Msgf("l4b: %v - %v", after, before)

				log.Debug().Msg("v2 after")
				for n := 0; n < after.Len(); n++ {
					dep := after.Index(n).String()
					log.Trace().Str("dep", dep).Msg("found dep, removing old connections")
					depSplit := strings.SplitN(dep, "::", 3)
					depGroup := depSplit[0]
					depType := depSplit[1]
					depName := depSplit[2]
					log.Trace().
						Str("depGroup", depGroup).
						Str("depType", depType).
						Str("depName", depName).
						Str("vGroup", vGroup).
						Str("vType", vType).
						Str("vName", vName).
						Msg("after loop")
					var aVertex, bVertex *gograph.Vertex[*LawNode]
					for _, vtx := range graph.GetAllVertices() {
						log.Debug().
							Interface("vertex", vtx).
							Str("group", vtx.Label().Group).
							Str("type", vtx.Label().Type).
							Str("name", vtx.Label().Name).
							Msgf("vertex: %v", vtx)
						if vtx.Label().Group == depGroup && vtx.Label().Type == depType && vtx.Label().Name == depName {
							log.Debug().Interface("vertex", vtx.Label().Name).Msg("found bvertex")
							bVertex = vtx
						}
						if vtx.Label().Group == vGroup && vtx.Label().Type == vType && vtx.Label().Name == vName {
							aVertex = vtx
						}
					}
					_, err := graph.AddEdge(bVertex, aVertex)
					if err != nil {
						log.Error().Err(err).
							Str("law name", vName).
							Str("law type", vType).
							Str("law group", vGroup).
							Msg("failed to add edge")

					}
					graph.RemoveEdges(graph.GetAllEdges(rootVertex, aVertex)...)
					log.Debug().
						Str("user", m.String()).
						Interface("aVertex", aVertex).
						Interface("bVertex", bVertex).
						Msgf("%v | %v", aVertex, bVertex)
				}

				// }
			}
		}
	}

	sorted, err := gograph.TopologySort(graph)
	if err != nil {
		log.Error().Err(err).Msg("failed to topo sort")
	}
	for _, v := range sorted {
		log.Trace().Msgf("(%v::%v::%v)", v.Label().Group, v.Label().Type, v.Label().Name)
	}

	return sorted, nil
}

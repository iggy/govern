package laws

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"dario.cat/mergo"
	"github.com/Masterminds/sprig/v3"
	"github.com/hmdsefi/gograph"
	"github.com/iggy/govern/pkg/facts"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
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

	laws := &Laws3{}
	// laws := NewLaws[string]()
	// laws := &Laws3{
	// 	struct{ Present []User }{
	// 		Present: []User{
	// 			{Name: "root"},
	// 			{Name: "iggy"},
	// 		},
	// 	},
	// }
	// // laws := map[string]map[string][]interface{}{}

	// yamlOut, _ := yaml.Marshal(laws)
	// log.Debug().Bytes("yaml", yamlOut).Msg("yaml Laws3")

	// return nil

	graph := gograph.New[*LawNode](gograph.Acyclic())
	rootVertex := gograph.NewVertex[*LawNode](&LawNode{&Root{Name: "root"}, "root", "root", "root"})
	log.Debug().Interface("rootv", rootVertex).Msg("I'm tired of having to constantly (un)comment this")
	// v2 := gograph.NewVertex[*LawNode](&LawNode{Group{Name: "iggy"}, "group"})
	// _, err := graph.AddEdge(v1, v2)
	// if err != nil {
	// 	log.Error().Interface("v1", v1).Interface("v2", v2).Msg("failed to add edge")
	// }

	// fi, err := os.Stat(path)
	// if err != nil {
	// 	log.Error().Err(err).Msg("failed to stat path to parse files")
	// }
	// if fi.IsDir() {
	// 	files =
	// }

	// TODO handle single files
	fileSystem := os.DirFS(path)

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

			loopLaws := &Laws3{}

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
			err = tmpl.Execute(&lawsWr, map[string]interface{}{"facts": facts.Facts}) // TODO pass more stuff to templates
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

	// for _, v := range laws.Users {
	// 	vtx := gograph.NewVertex[*LawNode](&LawNode{v, "user", v.Name})
	// 	graph.AddEdge(rootVertex, vtx)
	// }

	// for _, v := range laws.Packages {
	// 	vtx := gograph.NewVertex[*LawNode](&LawNode{v, "package", v.Name})
	// 	graph.AddEdge(rootVertex, vtx)
	// }

	// for _, v := range laws.Groups {
	// 	vtx := gograph.NewVertex[*LawNode](&LawNode{v, "group", v.Name})
	// 	graph.AddEdge(rootVertex, vtx)
	// }

	// for k, l := range laws {
	// 	log.
	// 		Debug().
	// 		Str("k", k).
	// 		Interface("l", l).
	// 		Interface("l type", reflect.TypeOf(l)).
	// 		Msg("laws loop")
	// 	for i, j := range l {
	// 		log.Debug().Str("i", i).Interface("j", j).Interface("j type", reflect.TypeOf(j)).Msg("laws loop")
	// 	}
	// }

	// log.
	// 	Debug().
	// 	Interface("user", laws["users"]["present"][1]).
	// 	Interface("type", reflect.TypeOf(laws["users"]["present"][1].(User))).
	// 	Msg("user type")
	// log.
	// 	Debug().
	// 	Interface("user", laws.Users.Present.users[0]).
	// 	// Interface("type", reflect.TypeOf(laws["users"]["present"][1].(User))).
	// 	Msg("user type")

	// for _, v := range laws.Users {
	// 	if v.After != nil {
	// 		for _, dep := range v.After {
	// 			log.Trace().Str("dep", dep).Msg("found dep, removing old connections")
	// 			log.Debug().Interface("v", v).Str("dep", dep).Msg("stuff")
	// 			depSplit := strings.SplitN(dep, "::", 2)
	// 			depType := depSplit[0]
	// 			depName := depSplit[1]
	// 			log.Trace().Str("type", depType).Str("name", depName).Msg("")
	// 			var aVertex, bVertex *gograph.Vertex[*LawNode]
	// 			for _, vtx := range graph.GetAllVertices() {
	// 				log.Debug().Interface("vertex", vtx).Str("type", vtx.Label().Type).Msgf("vertex: %v", vtx.Label().Name)
	// 				if vtx.Label().Type == depType && vtx.Label().Name == depName {
	// 					log.Debug().Interface("vertex", vtx.Label().Name).Msg("found vertex")
	// 					bVertex = vtx
	// 				}
	// 				if vtx.Label().Type == "user" && vtx.Label().Name == v.Name {
	// 					aVertex = vtx
	// 				}
	// 			}
	// 			// aVertex := graph.GetVertexByID(&LawNode{v, "user", v.Name})
	// 			graph.AddEdge(bVertex, aVertex)
	// 			graph.RemoveEdges(graph.GetAllEdges(rootVertex, aVertex)...)
	// 			log.Debug().Str("user", v.Name).Interface("aVertex", aVertex).Interface("bVertex", bVertex).Msg("")
	// 		}
	// 	}
	// 	// graph.AddEdge(v1, vtx)//
	// }

	// for _, v := range laws.Packages {
	// 	vtx := gograph.NewVertex[*LawNode](&LawNode{v, "pkg"})
	// 	graph.AddEdge(v1, vtx)
	// }

	// for _, v := range laws.Groups {
	// 	vtx := gograph.NewVertex[*LawNode](&LawNode{v, "group"})
	// 	graph.AddEdge(v1, vtx)
	// }

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
		// log.Debug().
		// 	Str("lg", lawGroup).
		// 	Interface("value", l1Values.Field(i)).
		// 	Interface("l2values", l2Values).
		// 	Interface("l2Types", l2Types).
		// 	Msgf("l1 kv: %v - %v", l1Values.Field(i).Interface(), l2Types)

		// this loop is over present/installed/running/etc
		for j := 0; j < l2Values.NumField(); j++ {
			lawsType := l2Types.Field(j).Name
			l3Values := reflect.ValueOf(l2Values.Field(j).Interface())
			// l3Types := l3Values.Type()
			// log.Debug().
			// 	Str("name", l2Types.Field(j).Name).
			// 	Str("lg", lawGroup).
			// 	Interface("lgs", lawGroupSetting).
			// 	Interface("value", l2Values.Field(j).Interface()).
			// 	// Interface("l3Values", l3Values).
			// 	// Interface("l3Types", l3Types).
			// 	Msgf("l2 kv: v: %v - t: %v", l3Values, l3Types)
			// log.Debug().Msgf("l3: %v", l3Values.Slice(0, l3Values.Len()))
			// v := l2Values.Field(j).Interface()

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
				// log.Debug().Msgf("l4a: %v - %v", m, m.Type())
				// log.Debug().Msgf("l4b: %v - %v", after, before)

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
				// Interface("l3Values", l3Values).
				// Interface("l3Types", l3Types).
				Msgf("l2 kv: v: %v - t: %v", l3Values, l3Types)
			log.Debug().Msgf("l3: %v", l3Values.Slice(0, l3Values.Len()))
			// v := l2Values.Field(j).Interface()

			// this loop is over the array of user/group/filetemplate/etc
			for k := 0; k < l3Values.Len(); k++ {
				// for _, k := range l3Values.Slice(0, l3Values.Len()) {
				m := l3Values.Index(k)
				before := m.Elem().FieldByName("Before")
				after := m.Elem().FieldByName("After")
				vGroup := strings.ToLower(lawGroup)
				vType := strings.ToLower(lawsType)
				vName := strings.ToLower(m.Elem().FieldByName("Name").String())
				log.Debug().Msgf("l4a: %v - %v", m, m.Type())
				log.Debug().Msgf("l4b: %v - %v", after, before)

				// vtx := gograph.NewVertex[*LawNode](&LawNode{m, lawGroup, lawGroupSetting})
				// vtx := gograph.NewVertex[*LawNode](
				// 	&LawNode{
				// 		Law:  m,
				// 		Type: vType,
				// 		Name: vName,
				// 	},
				// )
				// log.Debug().Str("type", vtx.Label().Type).Str("name", vtx.Label().Name).Msgf("l2 vtx: %v", vtx)
				// graph.AddEdge(rootVertex, vtx)
				// v2 := l3Values.Index(k).Interface().(User)
				// v2 := l3Values.Index(k).Elem().Convert(l3Values.Index(k).Type())
				// var v interface{}
				// switch vt := l3Values.Index(k).Interface().(type) {
				// case User:
				// 	v = l3Values.Index(k).Interface().(User)
				// 	log.Debug().Interface("switch type", vt).Interface("v", v).Msg("")
				// }
				// log.Debug().Interface("v", v).Msg("")
				// v := l3Values.Index(k).Interface().(CommonFields)
				// if v2.After != nil {
				log.Debug().Msg("v2 after")
				// for _, dep := range m.FieldByName("After").Slice(0, m.Len()) {
				for n := 0; n < after.Len(); n++ {
					dep := after.Index(n).String()
					log.Trace().Str("dep", dep).Msg("found dep, removing old connections")
					// log.Debug().Interface("v", v2).Str("dep", dep).Msg("stuff")
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
					// aVertex := graph.GetVertexByID(&LawNode{v, "user", v.Name})
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

	// log.Debug().Interface("graph", graph).Msgf("graph: %v", graph)

	sorted, err := gograph.TopologySort(graph)
	if err != nil {
		log.Error().Err(err).Msg("failed to topo sort")
	}
	for _, v := range sorted {
		// log.
		// 	Debug().
		// 	// Interface("sorted", sorted).
		// 	Interface("v", reflect.ValueOf(v.Label().Law).MethodByName("Ensure")).
		// 	Str("type", v.Label().Type).
		// 	Msg("")
		// fmt.Printf("(%v::%v::%v)-|-", v.Label().Group, v.Label().Type, v.Label().Name)
		// v.Label().Law.Ensure(true)
		log.Trace().Msgf("(%v::%v::%v)", v.Label().Group, v.Label().Type, v.Label().Name)
	}
	// fmt.Println()

	return sorted, nil
}

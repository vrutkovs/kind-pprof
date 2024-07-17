package main

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/pprof/profile"
	"github.com/inhies/go-bytesize"
)

type SampleValues struct {
	object        string
	alloc_objects int32
	alloc_space   int32
	inuse_objects int32
	inuse_space   int32
}

func (s *SampleValues) size() int32 {
	return s.alloc_space
}

func (s *SampleValues) humanSize() string {
	return bytesize.New(float64(s.size())).Format("%.2f ", "Mb", true)
}

func main() {
	if len(os.Args) != 2 {
		panic("invalid args")
	}
	source := os.Args[1]

	inbytes, err := os.ReadFile(filepath.Join(".", source))
	if err != nil {
		panic(err)
	}
	profile, err := profile.Parse(bytes.NewBuffer(inbytes))
	if err != nil {
		panic(err)
	}

	objs := map[string]SampleValues{}
	for _, sample := range profile.Sample {
		for _, location := range sample.Location {
			for _, line := range location.Line {
				funcName := line.Function.Name
				funcNameSplit := strings.Split(funcName, ".")
				if len(funcNameSplit) < 3 {
					continue
				}
				baseFuncName := funcNameSplit[len(funcNameSplit)-1]
				if baseFuncName != "Unmarshal" {
					continue
				}
				if funcNameSplit[0] != "k8s" {
					continue
				}
				objName := funcNameSplit[len(funcNameSplit)-2]
				if string(objName[0]) != "(" || string(objName[len(objName)-1]) != ")" {
					continue
				}
				if existing, ok := objs[objName]; !ok {
					objs[objName] = SampleValues{
						alloc_objects: int32(sample.Value[0]),
						alloc_space:   int32(sample.Value[1]),
						inuse_objects: int32(sample.Value[2]),
						inuse_space:   int32(sample.Value[3]),
					}
				} else {
					existing.alloc_objects += int32(sample.Value[0])
					existing.alloc_space += int32(sample.Value[1])
					existing.inuse_objects += int32(sample.Value[2])
					existing.inuse_space += int32(sample.Value[3])
				}
			}
		}
	}

	// Convert map into slice to get it sorted
	results := []SampleValues{}
	for k, v := range objs {
		v.object = k
		results = append(results, v)
	}
	// Sort slices
	slices.SortFunc(results,
		func(a, b SampleValues) int {
			return cmp.Compare(b.size(), a.size())
		})

	for _, v := range results {
		fmt.Printf("%s %s: alloc obj %d alloc space %d / inuse obj %d inuse space %d\n", v.object, v.humanSize(), v.alloc_objects, v.alloc_space, v.inuse_objects, v.inuse_space)
	}
}

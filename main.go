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
	object      string
	alloc       int32
	alloc_count int32
	space       int32
	bytes       int32
}

func (s *SampleValues) size() int32 {
	return s.alloc * s.alloc_count
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
				if strings.Contains(funcName, "k8s.io") && strings.Contains(funcName, "Unmarshal") {
					funcNameSplit := strings.Split(funcName, ".")
					if len(funcNameSplit) < 3 {
						continue
					}
					baseFuncName := funcNameSplit[len(funcNameSplit)-1]
					if baseFuncName != "Unmarshal" {
						continue
					}
					objName := funcNameSplit[len(funcNameSplit)-2]
					if string(objName[0]) != "(" || string(objName[len(objName)-1]) != ")" {
						continue
					}
					if existing, ok := objs[objName]; !ok {
						objs[objName] = SampleValues{
							alloc:       int32(sample.Value[0]),
							alloc_count: int32(sample.Value[1]),
							space:       int32(sample.Value[2]),
							bytes:       int32(sample.Value[3]),
						}
					} else {
						existing.alloc += int32(sample.Value[0])
						existing.alloc_count += int32(sample.Value[1])
						existing.space += int32(sample.Value[2])
						existing.bytes += int32(sample.Value[3])
					}
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
		fmt.Printf("%s %s: alloc %d num %d / in_use %d num %d\n", v.object, v.humanSize(), v.alloc, v.alloc_count, v.space, v.bytes)
	}
}

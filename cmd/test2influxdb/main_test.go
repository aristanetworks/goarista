// Copyright (c) 2018 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"os"
	"testing"
	"time"

	"github.com/aristanetworks/goarista/test"
	"github.com/influxdata/influxdb/client/v2"
)

func newPoint(t *testing.T, measurement string, tags map[string]string,
	fields map[string]interface{}, timeString string) *client.Point {
	t.Helper()
	timestamp, err := time.Parse(time.RFC3339Nano, timeString)
	if err != nil {
		t.Fatal(err)
	}
	p, err := client.NewPoint(measurement, tags, fields, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseTestOutput(t *testing.T) {
	// Verify tags and fields set by flags are set in records
	flagTags.Set("tag=foo")
	flagFields.Set("field=true")
	defer func() {
		flagTags = nil
		flagFields = nil
	}()

	f, err := os.Open("testdata/output.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	makeTags := func(pkg, resultType string) map[string]string {
		return map[string]string{"package": pkg, "type": resultType, "tag": "foo"}
	}
	makeFields := func(pass, elapsed float64, test string) map[string]interface{} {
		m := map[string]interface{}{"pass": pass, "elapsed": elapsed, "field": true}
		if test != "" {
			m["test"] = test
		}
		return m
	}

	expected := []*client.Point{
		newPoint(t,
			"result",
			makeTags("pkg/passed", "test"),
			makeFields(1, 0, "TestPass"),
			"2018-03-08T10:33:12.344165231-08:00",
		),
		newPoint(t,
			"result",
			makeTags("pkg/passed", "package"),
			makeFields(1, 0.013, ""),
			"2018-03-08T10:33:12.34533033-08:00",
		),
		newPoint(t,
			"result",
			makeTags("pkg/panic", "test"),
			makeFields(0, 600.029, "TestPanic"),
			"2018-03-08T10:33:20.272440286-08:00",
		),
		newPoint(t,
			"result",
			makeTags("pkg/failed", "test"),
			makeFields(0, 0.18, "TestFail"),
			"2018-03-08T10:33:27.158860934-08:00",
		),
		newPoint(t,
			"result",
			makeTags("pkg/failed", "package"),
			makeFields(0, 0.204, ""),
			"2018-03-08T10:33:27.161302093-08:00",
		),
		newPoint(t,
			"result",
			makeTags("pkg/panic", "package"),
			makeFields(0, 0, ""),
			"2018-03-08T10:33:20.273440286-08:00",
		),
	}

	batch, err := client.NewBatchPoints(client.BatchPointsConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := parseTestOutput(f, batch); err != nil {
		t.Fatal(err)
	}

	if diff := test.Diff(expected, batch.Points()); diff != "" {
		t.Errorf("unexpected diff: %s", diff)
	}
}

func TestTagsFlag(t *testing.T) {
	for tc, expected := range map[string]tags{
		"abc=def":         tags{tag{key: "abc", value: "def"}},
		"abc=def,ghi=klm": tags{tag{key: "abc", value: "def"}, tag{key: "ghi", value: "klm"}},
	} {
		t.Run(tc, func(t *testing.T) {
			var ts tags
			ts.Set(tc)
			if diff := test.Diff(expected, ts); diff != "" {
				t.Errorf("unexpected diff from Set: %s", diff)
			}

			if s := ts.String(); s != tc {
				t.Errorf("unexpected diff from String: %q vs. %q", tc, s)
			}
		})
	}
}

func TestFieldsFlag(t *testing.T) {
	for tc, expected := range map[string]fields{
		"str=abc":        fields{field{key: "str", value: "abc"}},
		"bool=true":      fields{field{key: "bool", value: true}},
		"bool=false":     fields{field{key: "bool", value: false}},
		"float64=42":     fields{field{key: "float64", value: float64(42)}},
		"float64=42.123": fields{field{key: "float64", value: float64(42.123)}},
		"int64=42i":      fields{field{key: "int64", value: int64(42)}},
		"str=abc,bool=true,float64=42,int64=42i": fields{field{key: "str", value: "abc"},
			field{key: "bool", value: true},
			field{key: "float64", value: float64(42)},
			field{key: "int64", value: int64(42)}},
	} {
		t.Run(tc, func(t *testing.T) {
			var fs fields
			fs.Set(tc)
			if diff := test.Diff(expected, fs); diff != "" {
				t.Errorf("unexpected diff from Set: %s", diff)
			}

			if s := fs.String(); s != tc {
				t.Errorf("unexpected diff from String: %q vs. %q", tc, s)
			}
		})
	}
}

func TestParseBenchmarkOutput(t *testing.T) {
	// Verify tags and fields set by flags are set in records
	flagTags.Set("tag=foo")
	flagFields.Set("field=true")
	defaultMeasurement := *flagMeasurement
	*flagMeasurement = "benchmarks"
	defer func() {
		flagTags = nil
		flagFields = nil
		*flagMeasurement = defaultMeasurement
	}()

	f, err := os.Open("testdata/bench-output.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	makeTags := func(pkg, benchmark string) map[string]string {
		return map[string]string{
			"package":   pkg,
			"benchmark": benchmark,
			"tag":       "foo",
		}
	}
	makeFields := func(nsPerOp, mbPerS, bPerOp, allocsPerOp float64) map[string]interface{} {
		m := map[string]interface{}{
			"field": true,
		}
		if nsPerOp > 0 {
			m[fieldNsPerOp] = nsPerOp
		}
		if mbPerS > 0 {
			m[fieldMBPerS] = mbPerS
		}
		if bPerOp > 0 {
			m[fieldAllocedBytesPerOp] = bPerOp
		}
		if allocsPerOp > 0 {
			m[fieldAllocsPerOp] = allocsPerOp
		}
		return m
	}

	expected := []*client.Point{
		newPoint(t,
			"benchmarks",
			makeTags("arista/pkg", "BenchmarkPassed-8"),
			makeFields(127, 0, 16, 1),
			"2018-11-08T15:53:12.935603594-08:00",
		),
		newPoint(t,
			"benchmarks",
			makeTags("arista/pkg/subpkg1", "BenchmarkLogged-8"),
			makeFields(120, 0, 16, 1),
			"2018-11-08T15:53:14.359792815-08:00",
		),
		newPoint(t,
			"benchmarks",
			makeTags("arista/pkg/subpkg2", "BenchmarkSetBytes-8"),
			makeFields(120, 8.31, 16, 1),
			"2018-11-08T15:53:15.717036333-08:00",
		),
		newPoint(t,
			"benchmarks",
			makeTags("arista/pkg/subpkg3", "BenchmarkWithSubs/sub_1-8"),
			makeFields(118, 0, 16, 1),
			"2018-11-08T15:53:17.952644273-08:00",
		),
		newPoint(t,
			"benchmarks",
			makeTags("arista/pkg/subpkg3", "BenchmarkWithSubs/sub_2-8"),
			makeFields(117, 0, 16, 1),
			"2018-11-08T15:53:20.443187742-08:00",
		),
	}

	batch, err := client.NewBatchPoints(client.BatchPointsConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := parseBenchmarkOutput(f, batch); err != nil {
		t.Fatal(err)
	}

	// parseBenchmarkOutput arranges the data in maps so the generated points
	// are in random order. Therefore, we're diffing as map instead of a slice
	pointsAsMap := func(points []*client.Point) map[string]*client.Point {
		m := make(map[string]*client.Point, len(points))
		for _, p := range points {
			m[p.String()] = p
		}
		return m
	}
	expectedMap := pointsAsMap(expected)
	actualMap := pointsAsMap(batch.Points())
	if diff := test.Diff(expectedMap, actualMap); diff != "" {
		t.Errorf("unexpected diff: %s\nexpected: %v\nactual: %v", diff, expectedMap, actualMap)
	}
}

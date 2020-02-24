// Copyright (c) 2018 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package influxlib

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func testFields(line string, fields map[string]interface{},
	t *testing.T) {
	for k, v := range fields {
		formatString := "%s=%v"

		if _, ok := v.(string); ok {
			formatString = "%s=%q"
		}
		expected := fmt.Sprintf(formatString, k, v)
		if !strings.Contains(line, expected) {
			t.Errorf("%s expected in %s", expected, line)
		}

	}
}

func testTags(line string, tags map[string]string,
	t *testing.T) {
	for k, v := range tags {
		expected := fmt.Sprintf("%s=%s", k, v)
		if !strings.Contains(line, expected) {
			t.Errorf("%s expected in %s", expected, line)
		}
	}
}

func TestBasicWrite(t *testing.T) {
	testConn, _ := NewMockConnection()

	measurement := "TestData"
	tags := map[string]string{
		"tag1": "Happy",
		"tag2": "Valentines",
		"tag3": "Day",
	}
	fields := map[string]interface{}{
		"Data1": 1234,
		"Data2": "apples",
		"Data3": 5.34,
	}

	err := testConn.WritePoint(measurement, tags, fields)
	if err != nil {
		t.Fatal(err)
	}

	line, err := GetTestBuffer(testConn)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(line, measurement) {
		t.Fatalf("expected to find prefix %s in %s", measurement, line)
	}
	testTags(line, tags, t)
	testFields(line, fields, t)
}

func TestConnectionToHostFailure(t *testing.T) {
	config := &InfluxConfig{
		Port:     8086,
		Protocol: HTTP,
		Database: "test",
	}
	config.Hostname = "this is fake.com"
	if _, err := Connect(config); err == nil {
		t.Fatal("got no error")
	}
	config.Hostname = "\\-Fake.Url.Com"
	if _, err := Connect(config); err == nil {
		t.Fatal("got no error")
	}
}

func TestWriteFailure(t *testing.T) {
	con, _ := NewMockConnection()

	measurement := "TestData"
	tags := map[string]string{
		"tag1": "hi",
	}
	data := map[string]interface{}{
		"Data1": "cats",
	}
	if err := con.WritePoint(measurement, tags, data); err != nil {
		t.Fatal(err)
	}

	fc, _ := con.Client.(*fakeClient)
	fc.failAll = true

	if err := con.WritePoint(measurement, tags, data); err == nil {
		t.Fatal("got no error")
	}
}

func TestQuery(t *testing.T) {
	query := "SELECT * FROM 'system' LIMIT 50;"
	con, _ := NewMockConnection()
	if _, err := con.Query(query); err != nil {
		t.Fatal(err)
	}
}

func TestAddAndWriteBatchPoints(t *testing.T) {
	testConn, _ := NewMockConnection()

	measurement := "TestData"
	points := []Point{
		Point{
			Measurement: measurement,
			Tags: map[string]string{
				"tag1": "Happy",
				"tag2": "Valentines",
				"tag3": "Day",
			},
			Fields: map[string]interface{}{
				"Data1": 1234,
				"Data2": "apples",
				"Data3": 5.34,
			},
			Timestamp: time.Now(),
		},
		Point{
			Measurement: measurement,
			Tags: map[string]string{
				"tag1": "Happy",
				"tag2": "New",
				"tag3": "Year",
			},
			Fields: map[string]interface{}{
				"Data1": 5678,
				"Data2": "bananas",
				"Data3": 3.14,
			},
			Timestamp: time.Now(),
		},
	}

	err := testConn.RecordBatchPoints(points)
	if err != nil {
		t.Fatal(err)
	}

	line, err := GetTestBuffer(testConn)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(line, measurement) {
		t.Fatalf("%s does not appear in %s", measurement, line)
	}
	for _, p := range points {
		testTags(line, p.Tags, t)
		testFields(line, p.Fields, t)
	}
}

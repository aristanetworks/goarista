// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"testing"

	"github.com/aristanetworks/goarista/test"
)

func TestConfig(t *testing.T) {
	cfg, err := loadConfig("/nonexistent.json")
	if err == nil {
		t.Fatal("Managed to load a nonexistent config!")
	}
	cfg, err = loadConfig("sampleconfig.json")
	if err != nil {
		t.Fatal("Failed to load config:", err)
	}

	testcases := []struct {
		path           string
		metric         string
		tags           map[string]string
		staticValueMap map[string]int64
	}{{
		path:           "/Sysdb/environment/cooling/status/fan/Fan1/1/speed/value",
		metric:         "eos.fanspeed.environment.fan.speed",
		tags:           map[string]string{"fan": "Fan1/1"},
		staticValueMap: map[string]int64{},
	}, {
		path: "/Sysdb/environment/power/status/powerSupply/PowerSupply2/" +
			"outputPower/value",
		metric:         "eos.powersensor.environment.power.output",
		tags:           map[string]string{"sensor": "PowerSupply2"},
		staticValueMap: map[string]int64{},
	}, {
		path: "/Sysdb/environment/power/status/voltageSensor/VoltageSensor23/" +
			"voltage/value",
		metric:         "eos.voltagesensor.environment.voltage",
		tags:           map[string]string{"sensor": "VoltageSensor23"},
		staticValueMap: map[string]int64{},
	}, {
		path: "/Sysdb/environment/power/status/currentSensor/CurrentSensorP2/1/" +
			"current/value",
		metric:         "eos.currentsensor.environment.current",
		tags:           map[string]string{"sensor": "CurrentSensorP2/1"},
		staticValueMap: map[string]int64{},
	}, {
		path: "/Sysdb/environment/temperature/status/tempSensor/" +
			"TempSensorP2/1/maxTemperature/value",
		metric:         "eos.tempsensor.environment.maxtemperature",
		tags:           map[string]string{"sensor": "TempSensorP2/1"},
		staticValueMap: map[string]int64{},
	}, {
		path: "/Sysdb/interface/counter/eth/lag/intfCounterDir/" +
			"Port-Channel201/intfCounter/current/statistics/outUcastPkts",
		metric: "eos.intfpktcounter.interface.pkt",
		tags: map[string]string{"intf": "Port-Channel201", "direction": "out",
			"type": "Ucast"},
		staticValueMap: map[string]int64{},
	}, {
		path: "/Sysdb/interface/counter/eth/slice/phy/1/intfCounterDir/" +
			"Ethernet42/intfCounter/current/statistics/inUcastPkts",
		metric:         "eos.intfpktcounter.interface.pkt",
		tags:           map[string]string{"intf": "Ethernet42", "direction": "in", "type": "Ucast"},
		staticValueMap: map[string]int64{},
	}, {
		path: "/Sysdb/interface/counter/eth/slice/phy/1/intfCounterDir/" +
			"Ethernet42/intfCounter/lastClear/statistics/inErrors",
		staticValueMap: map[string]int64{},
	}, {
		path: "/Sysdb/interface/counter/eth/slice/phy/1/intfCounterDir/" +
			"Ethernet42/intfCounter/current/ethStatistics/outPfcClassFrames",
		metric:         "eos.intfpfcclasscounter.interface.pfcclassframes",
		tags:           map[string]string{"intf": "Ethernet42", "direction": "out"},
		staticValueMap: map[string]int64{},
	}, {
		path: "/Sysdb/interface/status/eth/phy/slice/1/intfStatus/" +
			"Ethernet42/operStatus$",
		metric:         "eos.operstatus",
		tags:           map[string]string{"intf": "Ethernet42"},
		staticValueMap: map[string]int64{"intfOperUp": 1, "intfOperDown": 0, "default": 0},
	}}

	for i, tcase := range testcases {
		actualMetric, actualTags, staticValueMap := cfg.Match(tcase.path)
		if actualMetric != tcase.metric {
			t.Errorf("#%d expected metric %q but got %q", i, tcase.metric, actualMetric)
		}
		if d := test.Diff(tcase.tags, actualTags); actualMetric != "" && d != "" {
			t.Errorf("#%d expected tags %q but got %q: %s", i, tcase.tags, actualTags, d)
		}

		if d := test.Diff(tcase.staticValueMap, staticValueMap); d != "" {
			t.Errorf("#%d expected staticValueMap %q but got %q, %q", i, tcase.staticValueMap,
				staticValueMap, d)
		}

	}
}

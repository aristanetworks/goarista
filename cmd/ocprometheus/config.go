// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"fmt"
	"github.com/aristanetworks/glog"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
)

// Config is the representation of ocprometheus's YAML config file.
type Config struct {
	// Per-device labels.
	DeviceLabels map[string]prometheus.Labels

	// Prefixes to subscribe to.
	Subscriptions []string

	// Metrics to collect and how to munge them.
	Metrics []*MetricDef

	// Subscribed paths by their origin
	subsByOrigin map[string][]string

	//DescSubs  paths used
	DescriptionLabelSubscriptions []string `yaml:"description-label-subscriptions,omitempty"`
}

// MetricDef is the representation of a metric definiton in the config file.
type MetricDef struct {
	// Path is a regexp to match on the Update's full path.
	// The regexp must be a prefix match.
	// The regexp can define named capture groups to use as labels.
	Path string

	// Path compiled as a regexp.
	re *regexp.Regexp `deepequal:"ignore"`

	// Metric name.
	Name string

	// Metric help string.
	Help string

	// Label to store string values
	ValueLabel string

	// Default value to display for string values
	DefaultValue float64

	// Does the metric store a string value
	stringMetric bool

	// This map contains the metric descriptors for this metric for each device.
	devDesc map[string]*promDesc

	// This is the default metric descriptor for devices that don't have explicit descs.
	desc *promDesc
}

type promDesc struct {
	fqName        string
	help          string
	varLabels     []string
	devPermLabels map[string]string // required labels
}

// metricValues contains the values used in updating a metric
type metricValues struct {
	desc         *prometheus.Desc
	labels       []string
	defaultValue float64
	stringMetric bool
}

// Parses the config and creates the descriptors for each path and device.
func parseConfig(cfg []byte) (*Config, error) {
	config := &Config{
		DeviceLabels: make(map[string]prometheus.Labels),
	}
	if err := yaml.Unmarshal(cfg, config); err != nil {
		return nil, fmt.Errorf("Failed to parse config: %v", err)
	}

	config.subsByOrigin = make(map[string][]string)
	config.addSubscriptions(config.Subscriptions)
	descNodes := config.DescriptionLabelSubscriptions[:0]
	for _, p := range config.DescriptionLabelSubscriptions {
		if !strings.HasSuffix(p, "description") {
			glog.V(2).Infof("skipping %s as it is not a description node", p)
			continue
		}
		descNodes = append(descNodes, p)
	}
	config.DescriptionLabelSubscriptions = descNodes

	for _, def := range config.Metrics {
		def.re = regexp.MustCompile(def.Path)
		// Extract label names
		reNames := def.re.SubexpNames()[1:]
		labelNames := make([]string, len(reNames))
		for i, n := range reNames {
			labelNames[i] = n
			if n == "" {
				labelNames[i] = "unnamedLabel" + strconv.Itoa(i+1)
			}
		}
		if def.ValueLabel != "" {
			labelNames = append(labelNames, def.ValueLabel)
			def.stringMetric = true
		}
		// Create a default descriptor only if there aren't any per-device labels,
		// or if it's explicitly declared
		if len(config.DeviceLabels) == 0 || len(config.DeviceLabels["*"]) > 0 {
			def.desc = &promDesc{
				fqName:        def.Name,
				help:          def.Help,
				varLabels:     labelNames,
				devPermLabels: config.DeviceLabels["*"],
			}
		}
		// Add per-device descriptors
		def.devDesc = make(map[string]*promDesc)
		for device, labels := range config.DeviceLabels {
			if device == "*" {
				continue
			}
			def.devDesc[device] = &promDesc{
				fqName:        def.Name,
				help:          def.Help,
				varLabels:     labelNames,
				devPermLabels: labels,
			}
		}
	}

	return config, nil
}

// Returns a struct containing the descriptor corresponding to the device and path, labels
// extracted from the path, the default value for the metric and if it accepts string values.
// If the device and path doesn't match any metrics, returns nil.
func (c *Config) getMetricValues(s source) *metricValues {
	for _, def := range c.Metrics {
		if groups := def.re.FindStringSubmatch(s.path); groups != nil {
			if def.ValueLabel != "" {
				groups = append(groups, def.ValueLabel)
			}
			promdescVal, ok := def.devDesc[s.addr]
			if !ok {
				promdescVal = def.desc
			}

			desc := prometheus.NewDesc(promdescVal.fqName, promdescVal.help, promdescVal.varLabels,
				promdescVal.devPermLabels)
			return &metricValues{desc: desc, labels: groups[1:], defaultValue: def.DefaultValue,
				stringMetric: def.stringMetric}
		}
	}

	return nil
}

// Sends all the descriptors to the channel.
func (c *Config) getAllDescs(ch chan<- *prometheus.Desc) {
	for _, def := range c.Metrics {
		// Default descriptor might not be present
		if def.desc != nil {
			ch <- prometheus.NewDesc(def.desc.fqName, def.desc.help,
				def.desc.varLabels, def.desc.devPermLabels)
		}

		for _, desc := range def.devDesc {
			ch <- prometheus.NewDesc(desc.fqName, desc.help,
				desc.varLabels, desc.devPermLabels)
		}
	}
}

func (c *Config) addSubscriptions(subscriptions []string) {
	for _, sub := range subscriptions {
		parts := strings.SplitN(sub, ":", 2)
		if len(parts) == 1 || len(parts[0]) == 0 || parts[0][0] == '/' {
			c.subsByOrigin[""] = append(c.subsByOrigin[""], sub)
		} else {
			origin := parts[0]
			c.subsByOrigin[origin] = append(c.subsByOrigin[origin], parts[1])
		}
	}
}

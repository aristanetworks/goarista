// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"path"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/net/context"

	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/gnmi"
	gnmiUtils "github.com/aristanetworks/goarista/gnmi"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var labelRegex = regexp.MustCompile(`[a-zA-Z0-9_-]+`)

// A metric source.
type source struct {
	addr string
	path string
}

// Since the labels are fixed per-path and per-device we can cache them here,
// to avoid recomputing them.
type labelledMetric struct {
	metric       prometheus.Metric
	labels       []string
	defaultValue float64
	floatVal     float64
	stringMetric bool
}

type collector struct {
	// Protects access to metrics map
	m       sync.Mutex
	metrics map[source]*labelledMetric

	config            *Config
	descRegex         *regexp.Regexp
	descriptionLabels map[string]map[string]string
}

func newCollector(config *Config, descRegex *regexp.Regexp) *collector {
	return &collector{
		metrics:           make(map[source]*labelledMetric),
		config:            config,
		descriptionLabels: make(map[string]map[string]string),
		descRegex:         descRegex,
	}
}

// adds the label data to the map from the inital sync. No need to lock the map as we are not
// processing updates yet.
func (c *collector) addInitialDescriptionData(p *pb.Path, val string) {
	labels := extractLabelsFromDesc(val, c.descRegex)
	if len(labels) == 0 {
		return
	}
	c.descriptionLabels[gnmiUtils.StrPath(p)] = labels
}

// gets updates from the descriptin nodes and updates the map accordingly.
func (c *collector) deleteDescriptionTags(p *pb.Path) {
	c.m.Lock()
	defer c.m.Unlock()
	strP := gnmiUtils.StrPath(p)
	delete(c.descriptionLabels, strP)
	for s, m := range c.metrics {
		if !strings.Contains(s.path, strP) {
			continue
		}

		metric := c.config.getMetricValues(s, c.descriptionLabels)
		lm := prometheus.MustNewConstMetric(metric.desc, prometheus.GaugeValue, m.floatVal,
			metric.labels...)
		c.metrics[s].metric = lm
	}
}

func (c *collector) updateDescriptionTags(p *pb.Path, val string) {
	c.m.Lock()
	defer c.m.Unlock()

	strP := gnmiUtils.StrPath(p)
	labels := extractLabelsFromDesc(val, c.descRegex)
	c.descriptionLabels[strP] = labels

	for s, m := range c.metrics {
		if !strings.Contains(s.path, strP) {
			continue
		}

		met := c.config.getMetricValues(s, c.descriptionLabels)
		lm := prometheus.MustNewConstMetric(met.desc, prometheus.GaugeValue, m.floatVal,
			met.labels...)
		c.metrics[s].metric = lm
	}
}

func (c *collector) handleDescriptionNodes(ctx context.Context,
	respChan chan *pb.SubscribeResponse, wg *sync.WaitGroup) {
	var syncReceived bool
	defer func() {
		if !syncReceived {
			wg.Done()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case r := <-respChan:
			// if syncResponse has been received then start subscribing to metric paths
			if r.GetSyncResponse() {
				syncReceived = true
				wg.Done()
				continue
			}

			notif := r.GetUpdate()
			prefix := notif.GetPrefix()

			var err error
			if !syncReceived {
				// only updates will be present before syncResponse has been received
				for _, update := range notif.GetUpdate() {
					p := gnmi.JoinPaths(prefix, update.Path)
					p, err = getNearestList(p)
					if err != nil {
						glog.V(9).Infof("failed to parse description tags, got %s", err)
						continue
					}
					c.addInitialDescriptionData(p, update.GetVal().GetStringVal())
				}
				continue
			}

			// sync received, update data and regen tags if required.
			for _, d := range notif.GetDelete() {
				p := gnmi.JoinPaths(prefix, d)
				p, err = getNearestList(p)
				if err != nil {
					glog.V(9).Infof("failed to parse description tags, got %s", err)
					continue
				}
				c.deleteDescriptionTags(p)
			}

			for _, u := range notif.GetUpdate() {
				p := gnmi.JoinPaths(prefix, u.Path)
				p, err = getNearestList(p)
				if err != nil {
					glog.V(9).Infof("failed to parse description tags, got %s", err)
					continue
				}
				c.updateDescriptionTags(p, u.GetVal().GetStringVal())
			}
		}
	}
}

// using the default/user defined regex it extracts the labels from the description node value
func extractLabelsFromDesc(desc string, re *regexp.Regexp) map[string]string {
	labels := make(map[string]string)
	matches := re.FindAllStringSubmatch(desc, -1)
	glog.V(8).Infof("matched the following groups using the provided regex: %v", matches)

	if len(matches) > 2 {
		glog.V(8).Infof("received more than 2 match groups, got %v", matches)
	}
	for _, match := range matches {
		if match[2] == "" {
			if labelRegex.FindString(match[1]) != match[1] {
				glog.V(9).Infof("label %s did not match allowed regex "+
					"%s", match[1], labelRegex.String())
				continue
			}
			labels[match[1]] = "1"
			glog.V(9).Infof("found label %s=1", match[1])
			continue
		}

		match[2] = match[2][1:] // remove the equals sign
		if labelRegex.FindString(match[2]) != match[2] {
			glog.V(9).Infof("label %s did not match allowed regex %s",
				match[2], labelRegex.String())
			continue
		}
		labels[match[1]] = match[2]
		glog.V(9).Infof("found label %s%s", match[1], match[2])
	}
	return labels
}

// Process a notification and update or create the corresponding metrics.
func (c *collector) update(addr string, message proto.Message) {
	resp, ok := message.(*pb.SubscribeResponse)
	if !ok {
		glog.Errorf("Unexpected type of message: %T", message)
		return
	}

	notif := resp.GetUpdate()
	if notif == nil {
		return
	}

	device := strings.Split(addr, ":")[0]
	prefix := gnmi.StrPath(notif.Prefix)
	// Process deletes first
	for _, del := range notif.Delete {
		path := path.Join(prefix, gnmi.StrPath(del))
		key := source{addr: device, path: path}
		c.m.Lock()
		if _, ok := c.metrics[key]; ok {
			delete(c.metrics, key)
		} else {
			// TODO: replace this with a prefix tree
			p := path + "/"
			for k := range c.metrics {
				if k.addr == device && strings.HasPrefix(k.path, p) {
					delete(c.metrics, k)
				}
			}
		}
		c.m.Unlock()
	}

	// Process updates next
	for _, update := range notif.Update {
		path := path.Join(prefix, gnmi.StrPath(update.Path))
		value, suffix, ok := parseValue(update)
		if !ok {
			continue
		}

		var strUpdate bool
		var floatVal float64
		var strVal string

		switch v := value.(type) {
		case float64:
			strUpdate = false
			floatVal = v
		case string:
			strUpdate = true
			strVal = v
		}

		if suffix != "" {
			path += "/" + suffix
		}

		src := source{addr: device, path: path}
		c.m.Lock()
		// Use the cached labels and descriptor if available
		if m, ok := c.metrics[src]; ok {
			if strUpdate {
				// Skip string updates for non string metrics
				if !m.stringMetric {
					c.m.Unlock()
					continue
				}
				// Display a default value and replace the value label with the string value
				floatVal = m.defaultValue
				m.labels[len(m.labels)-1] = strVal
			}

			m.metric = prometheus.MustNewConstMetric(m.metric.Desc(), prometheus.GaugeValue,
				floatVal, m.labels...)
			m.floatVal = floatVal
			c.m.Unlock()
			continue
		}

		// Get the descriptor and labels for this source
		metric := c.config.getMetricValues(src, c.descriptionLabels)
		if metric == nil || metric.desc == nil {
			glog.V(8).Infof("Ignoring unmatched update %v at %s:%s with value %+v",
				update, device, path, value)
			c.m.Unlock()
			continue
		}

		if metric.stringMetric {
			if !strUpdate {
				// A float was parsed from the update, yet metric expects a string.
				// Store the float as a string.
				strVal = fmt.Sprintf("%.0f", floatVal)
			}
			// Display a default value and replace the value label with the string value
			floatVal = metric.defaultValue
			metric.labels[len(metric.labels)-1] = strVal
		}

		// Save the metric and labels in the cache
		lm := prometheus.MustNewConstMetric(metric.desc, prometheus.GaugeValue,
			floatVal, metric.labels...)
		c.metrics[src] = &labelledMetric{
			metric:       lm,
			floatVal:     floatVal,
			labels:       metric.labels,
			defaultValue: metric.defaultValue,
			stringMetric: metric.stringMetric,
		}
		c.m.Unlock()
	}
}

func getValue(intf interface{}) (interface{}, string, bool) {
	switch value := intf.(type) {
	// float64 or string expected as the return value
	case int64:
		return float64(value), "", true
	case uint64:
		return float64(value), "", true
	case float32:
		return float64(value), "", true
	case float64:
		return intf, "", true
	case *pb.Decimal64:
		val := gnmi.DecimalToFloat(value)
		if math.IsInf(val, 0) || math.IsNaN(val) {
			return 0, "", false
		}
		return val, "", true
	case json.Number:
		valFloat, err := value.Float64()
		if err != nil {
			return value, "", true
		}
		return valFloat, "", true
	case *anypb.Any:
		return value.String(), "", true
	case []interface{}:
		glog.V(9).Infof("skipping array value")
	case map[string]interface{}:
		if vIntf, ok := value["value"]; ok {
			res, suffix, ok := getValue(vIntf)
			if suffix != "" {
				return res, fmt.Sprintf("value/%s", suffix), ok
			}
			return res, "value", ok
		}
	case bool:
		if value {
			return float64(1), "", true
		}
		return float64(0), "", true
	case string:
		return value, "", true
	default:
		glog.V(9).Infof("Ignoring update with unexpected type: %T", value)
	}

	return 0, "", false
}

// parseValue takes in an update and parses a value and suffix
// Returns an interface that contains either a string or a float64 as well as a suffix
// Unparseable updates return (0, empty string, false)
func parseValue(update *pb.Update) (interface{}, string, bool) {
	intf, err := gnmi.ExtractValue(update)
	if err != nil {
		return 0, "", false
	}
	return getValue(intf)
}

// Describe implements prometheus.Collector interface
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	c.config.getAllDescs(ch)
}

// Collect implements prometheus.Collector interface
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	c.m.Lock()
	for _, m := range c.metrics {
		ch <- m.metric
	}
	c.m.Unlock()
}

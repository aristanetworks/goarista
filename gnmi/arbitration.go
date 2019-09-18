// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi_ext"
)

// ArbitrationExt takes a string representation of a master arbitration value
// (e.g. "23", "role:42") and return a *gnmi_ext.Extension.
func ArbitrationExt(s string) (*gnmi_ext.Extension, error) {
	if s == "" {
		return nil, nil
	}
	roleID, electionID, err := parseArbitrationString(s)
	if err != nil {
		return nil, err
	}
	arb := &gnmi_ext.MasterArbitration{
		Role:       &gnmi_ext.Role{Id: roleID},
		ElectionId: &gnmi_ext.Uint128{High: 0, Low: electionID},
	}
	ext := gnmi_ext.Extension_MasterArbitration{MasterArbitration: arb}
	return &gnmi_ext.Extension{Ext: &ext}, nil
}

// parseArbitrationString parses the supplied string and returns the role and election id
// values. Input is of the form [<role>:]<election_id>, where election_id is a uint64.
//
// Examples:
//  "1"
//  "admin:42"
func parseArbitrationString(s string) (string, uint64, error) {
	tokens := strings.Split(s, ":")
	switch len(tokens) {
	case 1: // just election id
		id, err := parseElectionID(tokens[0])
		return "", id, err
	case 2: // role and election id
		id, err := parseElectionID(tokens[1])
		return tokens[0], id, err
	}
	return "", 0, fmt.Errorf("badly formed arbitration id (%s)", s)
}

func parseElectionID(s string) (uint64, error) {
	id, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("badly formed arbitration id (%s)", s)
	}
	return id, nil
}

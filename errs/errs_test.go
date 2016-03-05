// Copyright (c) 2016 Arista Networks, Inc.  All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package errs_test

import (
	"encoding/xml"
	"testing"

	. "arista/errs"
)

var errorXMLCases = []struct {
	err error
	xml string
}{
	{
		err: NewInUse("resource", TypeApplication),
		xml: `<rpc-error><error-type>application</error-type>` +
			`<error-tag>in-use</error-tag><error-severity>error</error-severity>` +
			`<error-app-tag></error-app-tag><error-path></error-path>` +
			`<error-message>Resource &#34;resource&#34; is already in use` +
			`</error-message><error-description>The request requires a resource ` +
			`that already is in use.</error-description></rpc-error>`,
	},
}

func TestNetconfErrorMarshalXML(t *testing.T) {
	for _, tc := range errorXMLCases {
		b, err := xml.Marshal(tc.err)
		if err != nil {
			t.Errorf("unexpected error Marshaling error %#v", tc.err)
		} else if tc.xml != string(b) {
			t.Errorf("want: %s\ngot: %s", tc.xml, string(b))
		}
	}
}

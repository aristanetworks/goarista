// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path"

	pb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
)

// Get sents a GetRequest to the given client.
func Get(ctx context.Context, client pb.GNMIClient, paths [][]string) error {
	req, err := NewGetRequest(paths)
	if err != nil {
		return err
	}
	resp, err := client.Get(ctx, req)
	if err != nil {
		return err
	}
	for _, notif := range resp.Notification {
		for _, update := range notif.Update {
			fmt.Printf("%s:\n", StrPath(update.Path))
			fmt.Println(strUpdateVal(update))
		}
	}
	return nil
}

// Capabilities retuns the capabilities of the client.
func Capabilities(ctx context.Context, client pb.GNMIClient) error {
	resp, err := client.Capabilities(ctx, &pb.CapabilityRequest{})
	if err != nil {
		return err
	}
	fmt.Printf("Version: %s\n", resp.GNMIVersion)
	for _, mod := range resp.SupportedModels {
		fmt.Printf("SupportedModel: %s\n", mod)
	}
	for _, enc := range resp.SupportedEncodings {
		fmt.Printf("SupportedEncoding: %s\n", enc)
	}
	return nil
}

// val may be a path to a file or it may be json. First see if it is a
// file, if so return its contents, otherwise return val
func extractJSON(val string) []byte {
	jsonBytes, err := ioutil.ReadFile(val)
	if err != nil {
		jsonBytes = []byte(val)
	}
	return jsonBytes
}

// strUpdateVal will return a string representing the value within the supplied update
func strUpdateVal(u *pb.Update) string {
	if u.Value != nil {
		return string(u.Value.Value) // Backwards compatibility with pre-v0.4 gnmi
	}
	return strVal(u.Val)
}

// strVal will return a string representing the supplied value
func strVal(val *pb.TypedValue) string {
	switch v := val.GetValue().(type) {
	case *pb.TypedValue_StringVal:
		return v.StringVal
	case *pb.TypedValue_JsonIetfVal:
		return string(v.JsonIetfVal)
	case *pb.TypedValue_IntVal:
		return fmt.Sprintf("%v", v.IntVal)
	case *pb.TypedValue_UintVal:
		return fmt.Sprintf("%v", v.UintVal)
	case *pb.TypedValue_BoolVal:
		return fmt.Sprintf("%v", v.BoolVal)
	case *pb.TypedValue_BytesVal:
		return string(v.BytesVal)
	case *pb.TypedValue_DecimalVal:
		return strDecimal64(v.DecimalVal)
	case *pb.TypedValue_LeaflistVal:
		return strLeaflist(v.LeaflistVal)
	default:
		panic(v)
	}
}

func strDecimal64(d *pb.Decimal64) string {
	var i, frac uint64
	if d.Precision > 0 {
		div := uint64(10)
		it := d.Precision - 1
		for it > 0 {
			div *= 10
			it--
		}
		i = d.Digits / div
		frac = d.Digits % div
	} else {
		i = d.Digits
	}
	return fmt.Sprintf("%d.%d", i, frac)
}

// strLeafList builds a human-readable form of a leaf-list. e.g. [1,2,3] or [a,b,c]
func strLeaflist(v *pb.ScalarArray) string {
	s := make([]string, 0, len(v.Element))
	sz := 2 // []

	// convert arbitrary TypedValues to string form
	for _, elm := range v.Element {
		str := strVal(elm)
		s = append(s, str)
		sz += len(str) + 1 // %v + ,
	}

	b := make([]byte, sz)
	buf := bytes.NewBuffer(b)

	buf.WriteRune('[')
	for i := range v.Element {
		buf.WriteString(s[i])
		if i < len(v.Element)-1 {
			buf.WriteRune(',')
		}
	}
	buf.WriteRune(']')
	return buf.String()
}

func update(p *pb.Path, val string) *pb.Update {
	var v *pb.TypedValue
	switch p.Origin {
	case "":
		v = &pb.TypedValue{
			Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: extractJSON(val)}}
	case "cli":
		v = &pb.TypedValue{
			Value: &pb.TypedValue_AsciiVal{AsciiVal: val}}
	default:
		panic(fmt.Errorf("unexpected origin: %q", p.Origin))
	}

	return &pb.Update{Path: p, Val: v}
}

// Operation describes an gNMI operation.
type Operation struct {
	Type string
	Path []string
	Val  string
}

func newSetRequest(setOps []*Operation) (*pb.SetRequest, error) {
	req := &pb.SetRequest{}
	for _, op := range setOps {
		p, err := ParseGNMIElements(op.Path)
		if err != nil {
			return nil, err
		}

		switch op.Type {
		case "delete":
			req.Delete = append(req.Delete, p)
		case "update":
			req.Update = append(req.Update, update(p, op.Val))
		case "replace":
			req.Replace = append(req.Replace, update(p, op.Val))
		}
	}
	return req, nil
}

// Set sends a SetRequest to the given client.
func Set(ctx context.Context, client pb.GNMIClient, setOps []*Operation) error {
	req, err := newSetRequest(setOps)
	if err != nil {
		return err
	}
	resp, err := client.Set(ctx, req)
	if err != nil {
		return err
	}
	if resp.Message != nil && codes.Code(resp.Message.Code) != codes.OK {
		return errors.New(resp.Message.Message)
	}
	// TODO: Iterate over SetResponse.Response for more detailed error message?

	return nil
}

// Subscribe sends a SubscribeRequest to the given client.
func Subscribe(ctx context.Context, client pb.GNMIClient, paths [][]string,
	respChan chan<- *pb.SubscribeResponse, errChan chan<- error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := client.Subscribe(ctx)
	if err != nil {
		errChan <- err
		return
	}
	req, err := NewSubscribeRequest(paths)
	if err != nil {
		errChan <- err
		return
	}
	if err := stream.Send(req); err != nil {
		errChan <- err
		return
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return
			}
			errChan <- err
			return
		}
		respChan <- resp
	}
}

// LogSubscribeResponse logs update responses to stderr.
func LogSubscribeResponse(response *pb.SubscribeResponse) error {
	switch resp := response.Response.(type) {
	case *pb.SubscribeResponse_Error:
		return errors.New(resp.Error.Message)
	case *pb.SubscribeResponse_SyncResponse:
		if !resp.SyncResponse {
			return errors.New("initial sync failed")
		}
	case *pb.SubscribeResponse_Update:
		prefix := StrPath(resp.Update.Prefix)
		for _, update := range resp.Update.Update {
			fmt.Printf("%s = %s\n", path.Join(prefix, StrPath(update.Path)),
				strUpdateVal(update))
		}
	}
	return nil
}

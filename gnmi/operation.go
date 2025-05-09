// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"google.golang.org/grpc/codes"
)

// GetWithRequest takes a fully formed GetRequest, performs the Get,
// and displays any response.
func GetWithRequest(ctx context.Context, client pb.GNMIClient,
	req *pb.GetRequest) error {
	resp, err := client.Get(ctx, req)
	if err != nil {
		return err
	}
	for _, notif := range resp.Notification {
		prefix := StrPath(notif.Prefix)
		for _, update := range notif.Update {
			fmt.Printf("%s:\n", path.Join(prefix, StrPath(update.Path)))
			fmt.Println(StrUpdateVal(update))
		}
	}
	return nil
}

// Get sends a GetRequest to the given client.
func Get(ctx context.Context, client pb.GNMIClient, paths [][]string,
	origin string) error {
	req, err := NewGetRequest(paths, origin)
	if err != nil {
		return err
	}
	return GetWithRequest(ctx, client, req)
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
func extractContent(val string, origin string) []byte {
	if jsonBytes, err := ioutil.ReadFile(val); err == nil {
		return jsonBytes
	}
	// for CLI commands we don't need to add the outer quotes
	if origin == "cli" {
		return []byte(val)
	}
	// Best effort check if the value might a string literal, in which
	// case wrap it in quotes. This is to allow a user to do:
	//   gnmi update ../hostname host1234
	//   gnmi update ../description 'This is a description'
	// instead of forcing them to quote the string:
	//   gnmi update ../hostname '"host1234"'
	//   gnmi update ../description '"This is a description"'
	maybeUnquotedStringLiteral := func(s string) bool {
		if s == "true" || s == "false" || s == "null" || // JSON reserved words
			strings.ContainsAny(s, `"'{}[]`) { // Already quoted or is a JSON object or array
			return false
		} else if _, err := strconv.ParseInt(s, 0, 32); err == nil {
			// Integer. Using byte size of 32 because larger integer
			// types are supposed to be sent as strings in JSON.
			return false
		} else if _, err := strconv.ParseFloat(s, 64); err == nil {
			// Float
			return false
		}

		return true
	}
	if maybeUnquotedStringLiteral(val) {
		out := make([]byte, len(val)+2)
		out[0] = '"'
		copy(out[1:], val)
		out[len(out)-1] = '"'
		return out
	}
	return []byte(val)
}

// StrUpdateVal will return a string representing the value within the supplied update
func StrUpdateVal(u *pb.Update) string {
	return strUpdateVal(u, false)
}

// StrUpdateValCompactJSON will return a string representing the value within the supplied
// update. If the value is a JSON value, a non-indented JSON string will be returned.
func StrUpdateValCompactJSON(u *pb.Update) string {
	return strUpdateVal(u, true)
}

func strUpdateVal(u *pb.Update, alwaysCompactJSON bool) string {
	if u.Value != nil {
		// Backwards compatibility with pre-v0.4 gnmi
		switch u.Value.Type {
		case pb.Encoding_JSON, pb.Encoding_JSON_IETF:
			return strJSON(u.Value.Value, alwaysCompactJSON)
		case pb.Encoding_BYTES, pb.Encoding_PROTO:
			return base64.StdEncoding.EncodeToString(u.Value.Value)
		case pb.Encoding_ASCII:
			return string(u.Value.Value)
		default:
			return string(u.Value.Value)
		}
	}
	return strVal(u.Val, alwaysCompactJSON)
}

// StrVal will return a string representing the supplied value
func StrVal(val *pb.TypedValue) string {
	return strVal(val, false)
}

// StrValCompactJSON will return a string representing the supplied value. If the value
// is a JSON value, a non-indented JSON string will be returned.
func StrValCompactJSON(val *pb.TypedValue) string {
	return strVal(val, true)
}

func strVal(val *pb.TypedValue, alwaysCompactJSON bool) string {
	switch v := val.GetValue().(type) {
	case *pb.TypedValue_StringVal:
		return v.StringVal
	case *pb.TypedValue_JsonIetfVal:
		return strJSON(v.JsonIetfVal, alwaysCompactJSON)
	case *pb.TypedValue_JsonVal:
		return strJSON(v.JsonVal, alwaysCompactJSON)
	case *pb.TypedValue_IntVal:
		return strconv.FormatInt(v.IntVal, 10)
	case *pb.TypedValue_UintVal:
		return strconv.FormatUint(v.UintVal, 10)
	case *pb.TypedValue_BoolVal:
		return strconv.FormatBool(v.BoolVal)
	case *pb.TypedValue_BytesVal:
		return base64.StdEncoding.EncodeToString(v.BytesVal)
	case *pb.TypedValue_DecimalVal:
		return strDecimal64(v.DecimalVal)
	case *pb.TypedValue_FloatVal:
		return strconv.FormatFloat(float64(v.FloatVal), 'g', -1, 32)
	case *pb.TypedValue_DoubleVal:
		return strconv.FormatFloat(float64(v.DoubleVal), 'g', -1, 64)
	case *pb.TypedValue_LeaflistVal:
		return strLeaflist(v.LeaflistVal)
	case *pb.TypedValue_AsciiVal:
		return v.AsciiVal
	case *pb.TypedValue_AnyVal:
		return v.AnyVal.String()
	case *pb.TypedValue_ProtoBytes:
		return base64.StdEncoding.EncodeToString(v.ProtoBytes)
	case nil:
		return ""
	default:
		panic(v)
	}
}

func strJSON(inJSON []byte, alwaysCompactJSON bool) string {
	var (
		out bytes.Buffer
		err error
	)
	// Check for ',' as simple heuristic on whether to expand JSON
	// onto multiple lines, or compact it to a single line.
	if !alwaysCompactJSON && bytes.Contains(inJSON, []byte{','}) {
		err = json.Indent(&out, inJSON, "", "  ")
	} else {
		err = json.Compact(&out, inJSON)
	}
	if err != nil {
		return fmt.Sprintf("(error unmarshalling json: %s)\n", err) + string(inJSON)
	}
	return out.String()
}

func strDecimal64(d *pb.Decimal64) string {
	var i, frac int64
	if d.Precision > 0 {
		div := int64(10)
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
	format := "%d.%0*d"
	if frac < 0 {
		if i == 0 {
			// The integer part doesn't provide the necessary minus sign.
			format = "-" + format
		}
		frac = -frac
	}
	return fmt.Sprintf(format, i, int(d.Precision), frac)
}

// strLeafList builds a human-readable form of a leaf-list. e.g. [1, 2, 3] or [a, b, c]
func strLeaflist(v *pb.ScalarArray) string {
	var b strings.Builder
	b.WriteByte('[')

	for i, elm := range v.Element {
		b.WriteString(StrVal(elm))
		if i < len(v.Element)-1 {
			b.WriteString(", ")
		}
	}

	b.WriteByte(']')
	return b.String()
}

// TypedValue marshals an interface into a gNMI TypedValue value
func TypedValue(val interface{}) *pb.TypedValue {
	// TODO: handle more types:
	// maps
	// key.Key
	// key.Map
	// ... etc
	switch v := val.(type) {
	case *pb.TypedValue:
		return v
	case string:
		return &pb.TypedValue{Value: &pb.TypedValue_StringVal{StringVal: v}}
	case int:
		return &pb.TypedValue{Value: &pb.TypedValue_IntVal{IntVal: int64(v)}}
	case int8:
		return &pb.TypedValue{Value: &pb.TypedValue_IntVal{IntVal: int64(v)}}
	case int16:
		return &pb.TypedValue{Value: &pb.TypedValue_IntVal{IntVal: int64(v)}}
	case int32:
		return &pb.TypedValue{Value: &pb.TypedValue_IntVal{IntVal: int64(v)}}
	case int64:
		return &pb.TypedValue{Value: &pb.TypedValue_IntVal{IntVal: v}}
	case uint:
		return &pb.TypedValue{Value: &pb.TypedValue_UintVal{UintVal: uint64(v)}}
	case uint8:
		return &pb.TypedValue{Value: &pb.TypedValue_UintVal{UintVal: uint64(v)}}
	case uint16:
		return &pb.TypedValue{Value: &pb.TypedValue_UintVal{UintVal: uint64(v)}}
	case uint32:
		return &pb.TypedValue{Value: &pb.TypedValue_UintVal{UintVal: uint64(v)}}
	case uint64:
		return &pb.TypedValue{Value: &pb.TypedValue_UintVal{UintVal: v}}
	case bool:
		return &pb.TypedValue{Value: &pb.TypedValue_BoolVal{BoolVal: v}}
	case float32:
		return &pb.TypedValue{Value: &pb.TypedValue_FloatVal{FloatVal: v}}
	case float64:
		return &pb.TypedValue{Value: &pb.TypedValue_DoubleVal{DoubleVal: v}}
	case []byte:
		return &pb.TypedValue{Value: &pb.TypedValue_BytesVal{BytesVal: v}}
	case []interface{}:
		gnmiElems := make([]*pb.TypedValue, len(v))
		for i, elem := range v {
			gnmiElems[i] = TypedValue(elem)
		}
		return &pb.TypedValue{
			Value: &pb.TypedValue_LeaflistVal{
				LeaflistVal: &pb.ScalarArray{
					Element: gnmiElems,
				}}}
	default:
		panic(fmt.Sprintf("unexpected type %T for value %v", val, val))
	}
}

// ExtractValue pulls a value out of a gNMI Update, parsing JSON if present.
// Possible return types:
//
//	string
//	int64
//	uint64
//	bool
//	[]byte
//	float32
//	*gnmi.Decimal64
//	json.Number
//	*any.Any
//	[]interface{}
//	map[string]interface{}
func ExtractValue(update *pb.Update) (interface{}, error) {
	var i interface{}
	var err error
	if update == nil {
		return nil, fmt.Errorf("empty update")
	}
	if update.Val != nil {
		i, err = extractValueV04(update.Val)
	} else if update.Value != nil {
		i, err = extractValueV03(update.Value)
	}
	return i, err
}

func extractValueV04(val *pb.TypedValue) (interface{}, error) {
	switch v := val.Value.(type) {
	case *pb.TypedValue_StringVal:
		return v.StringVal, nil
	case *pb.TypedValue_IntVal:
		return v.IntVal, nil
	case *pb.TypedValue_UintVal:
		return v.UintVal, nil
	case *pb.TypedValue_BoolVal:
		return v.BoolVal, nil
	case *pb.TypedValue_BytesVal:
		return v.BytesVal, nil
	case *pb.TypedValue_FloatVal:
		return v.FloatVal, nil
	case *pb.TypedValue_DoubleVal:
		return v.DoubleVal, nil
	case *pb.TypedValue_DecimalVal:
		return v.DecimalVal, nil
	case *pb.TypedValue_LeaflistVal:
		elementList := v.LeaflistVal.Element
		l := make([]interface{}, len(elementList))
		for i, element := range elementList {
			el, err := extractValueV04(element)
			if err != nil {
				return nil, err
			}
			l[i] = el
		}
		return l, nil
	case *pb.TypedValue_AnyVal:
		return v.AnyVal, nil
	case *pb.TypedValue_JsonVal:
		return decode(v.JsonVal)
	case *pb.TypedValue_JsonIetfVal:
		return decode(v.JsonIetfVal)
	case *pb.TypedValue_AsciiVal:
		return v.AsciiVal, nil
	case *pb.TypedValue_ProtoBytes:
		return v.ProtoBytes, nil
	}
	return nil, fmt.Errorf("unhandled type of value %v", val.GetValue())
}

func extractValueV03(val *pb.Value) (interface{}, error) {
	switch val.Type {
	case pb.Encoding_JSON, pb.Encoding_JSON_IETF:
		return decode(val.Value)
	case pb.Encoding_BYTES, pb.Encoding_PROTO:
		return val.Value, nil
	case pb.Encoding_ASCII:
		return string(val.Value), nil
	}
	return nil, fmt.Errorf("unhandled type of value %v", val.GetValue())
}

func decode(byteArr []byte) (interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(byteArr))
	decoder.UseNumber()
	var value interface{}
	err := decoder.Decode(&value)
	return value, err
}

// DecimalToFloat converts a gNMI Decimal64 to a float64
func DecimalToFloat(dec *pb.Decimal64) float64 {
	return float64(dec.Digits) / math.Pow10(int(dec.Precision))
}

func update(p *pb.Path, val string) (*pb.Update, error) {
	var v *pb.TypedValue
	switch p.Origin {
	case "", "openconfig":
		v = &pb.TypedValue{
			Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: extractContent(val, p.Origin)}}
	case "eos_native":
		v = &pb.TypedValue{
			Value: &pb.TypedValue_JsonVal{JsonVal: extractContent(val, p.Origin)}}
	case "cli", "test-regen-cli":
		v = &pb.TypedValue{
			Value: &pb.TypedValue_AsciiVal{AsciiVal: string(extractContent(val, p.Origin))}}
	case "p4_config":
		b, err := ioutil.ReadFile(val)
		if err != nil {
			return nil, err
		}
		v = &pb.TypedValue{
			Value: &pb.TypedValue_ProtoBytes{ProtoBytes: b}}
	default:
		return nil, fmt.Errorf("unexpected origin: %q", p.Origin)
	}

	return &pb.Update{Path: p, Val: v}, nil
}

// Operation describes an gNMI operation.
type Operation struct {
	Type   string
	Origin string
	Target string
	Path   []string
	Val    string
}

func newSetRequest(setOps []*Operation, exts ...*gnmi_ext.Extension) (*pb.SetRequest, error) {
	req := &pb.SetRequest{}
	for _, op := range setOps {
		p, err := ParseGNMIElements(op.Path)
		if err != nil {
			return nil, err
		}
		p.Origin = op.Origin

		// Target must apply to the entire SetRequest.
		if op.Target != "" {
			req.Prefix = &pb.Path{
				Target: op.Target,
			}
		}

		switch op.Type {
		case "delete":
			req.Delete = append(req.Delete, p)
		case "update":
			u, err := update(p, op.Val)
			if err != nil {
				return nil, err
			}
			req.Update = append(req.Update, u)
		case "replace":
			u, err := update(p, op.Val)
			if err != nil {
				return nil, err
			}
			req.Replace = append(req.Replace, u)
		case "union_replace":
			u, err := update(p, op.Val)
			if err != nil {
				return nil, err
			}
			req.UnionReplace = append(req.UnionReplace, u)
		}
	}
	for _, ext := range exts {
		req.Extension = append(req.Extension, ext)
	}
	return req, nil
}

// Set sends a SetRequest to the given client.
func Set(ctx context.Context, client pb.GNMIClient, setOps []*Operation,
	exts ...*gnmi_ext.Extension) error {
	req, err := newSetRequest(setOps, exts...)
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
	return nil
}

// Subscribe sends a SubscribeRequest to the given client.
// Deprecated: Use SubscribeErr instead.
func Subscribe(ctx context.Context, client pb.GNMIClient, subscribeOptions *SubscribeOptions,
	respChan chan<- *pb.SubscribeResponse, errChan chan<- error) {
	defer close(errChan)
	if err := SubscribeErr(ctx, client, subscribeOptions, respChan); err != nil {
		errChan <- err
	}
}

// SubscribeErr makes a gNMI.Subscribe call and writes the responses
// to the respChan. Before returning respChan will be closed.
func SubscribeErr(ctx context.Context, client pb.GNMIClient, subscribeOptions *SubscribeOptions,
	respChan chan<- *pb.SubscribeResponse) error {
	req, err := NewSubscribeRequest(subscribeOptions)
	if err != nil {
		return err
	}
	return SubscribeWithRequest(ctx, client, req, respChan)
}

// SubscribeWithRequest calls gNMI.Subscribe with the SubscribeRequest.
func SubscribeWithRequest(ctx context.Context, client pb.GNMIClient, req *pb.SubscribeRequest,
	respChan chan<- *pb.SubscribeResponse) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(respChan)

	stream, err := client.Subscribe(ctx)
	if err != nil {
		return err
	}
	if err := stream.Send(req); err != nil {
		return err
	}
	if req.GetSubscribe().GetMode() != pb.SubscriptionList_POLL {
		// Non polling subscriptions are not expected to submit any other messages to server.
		if err := stream.CloseSend(); err != nil {
			return err
		}
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		select {
		case respChan <- resp:
		case <-ctx.Done():
			return ctx.Err()
		}

		// For POLL subscriptions, initiate a poll request by pressing ENTER
		if req.GetSubscribe().GetMode() == pb.SubscriptionList_POLL {
			switch resp.Response.(type) {
			case *pb.SubscribeResponse_SyncResponse:
				fmt.Print("Press ENTER to send a poll request: ")
				reader := bufio.NewReader(os.Stdin)
				reader.ReadString('\n')

				pollReq := &pb.SubscribeRequest{
					Request: &pb.SubscribeRequest_Poll{
						Poll: &pb.Poll{},
					},
				}
				if err := stream.Send(pollReq); err != nil {
					return err
				}
			}
		}
	}
}

const rfc3339NanoKeepTrailingZeros = "2006-01-02T15:04:05.000000000Z07:00"

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
		t := time.Unix(0, resp.Update.Timestamp).UTC()
		prefix := StrPath(resp.Update.Prefix)
		var target string
		if t := resp.Update.Prefix.GetTarget(); t != "" {
			target = "(" + t + ") "
		}
		for _, del := range resp.Update.Delete {
			fmt.Printf("[%s] %sDeleted %s\n", t.Format(rfc3339NanoKeepTrailingZeros),
				target,
				path.Join(prefix, StrPath(del)))
		}
		for _, update := range resp.Update.Update {
			fmt.Printf("[%s] %s%s = %s\n", t.Format(rfc3339NanoKeepTrailingZeros),
				target,
				path.Join(prefix, StrPath(update.Path)),
				StrUpdateVal(update))
		}
	}
	return nil
}

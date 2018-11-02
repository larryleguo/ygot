// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ytypes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/kylelemons/godebug/pretty"
	"github.com/openconfig/gnmi/errdiff"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/util"
	"github.com/openconfig/ygot/ygot"
)

var validLeafListSchema = &yang.Entry{
	Name:     "valid-leaf-list-schema",
	Kind:     yang.LeafEntry,
	Type:     &yang.YangType{Kind: yang.Ystring},
	ListAttr: &yang.ListAttr{MinElements: &yang.Value{Name: "0"}},
}

func TestValidateLeafListSchema(t *testing.T) {
	tests := []struct {
		desc    string
		schema  *yang.Entry
		wantErr bool
	}{
		{
			desc:   "success",
			schema: validLeafListSchema,
		},
		{
			desc:    "nil schema",
			schema:  nil,
			wantErr: true,
		},
		{
			desc:    "nil schema type",
			schema:  &yang.Entry{Name: "nil-type-schema", Type: nil},
			wantErr: true,
		},
		{
			desc: "invalid leaf-list schema - contains empty",
			schema: &yang.Entry{
				Name:     "invalid-leaflist",
				Kind:     yang.LeafEntry,
				Type:     &yang.YangType{Kind: yang.Yempty},
				ListAttr: &yang.ListAttr{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := validateLeafListSchema(tt.schema)
			if got, want := (err != nil), tt.wantErr; got != want {
				t.Errorf("%s: validateListSchema(%v) got error: %v, want error? %v", tt.desc, tt.schema, err, tt.wantErr)
			}
			testErrLog(t, tt.desc, err)
		})
	}
}

func TestValidateLeafList(t *testing.T) {
	leafListSchema := &yang.Entry{
		Kind:     yang.LeafEntry,
		ListAttr: &yang.ListAttr{MinElements: &yang.Value{Name: "0"}},
		Type:     &yang.YangType{Kind: yang.Ystring},
		Name:     "leaf-list-schema",
	}
	tests := []struct {
		desc    string
		schema  *yang.Entry
		val     interface{}
		wantErr string
	}{
		{
			desc:   "success nil value",
			schema: leafListSchema,
			val:    nil,
		},
		{
			desc:   "success",
			schema: leafListSchema,
			val:    []string{"test1", "test2"},
		},
		{
			desc:    "nil schema",
			schema:  nil,
			val:     []string{"test1"},
			wantErr: `nil schema for type []string, value [test1]`,
		},
		{
			desc:    "bad struct fields",
			schema:  leafListSchema,
			val:     []int32{1},
			wantErr: `non string type int32 with value 1 for schema leaf-list-schema`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			errs := Validate(tt.schema, tt.val)
			if got, want := errs.String(), tt.wantErr; got != want {
				t.Errorf("%s: Validate(%v) got error: %v, want error: %v", tt.desc, tt.val, got, want)
			}
			testErrLog(t, tt.desc, errs)
		})
	}

	// nil value
	if got := validateLeafList(nil, nil); got != nil {
		t.Errorf("nil value: got error: %v, want error: nil", got)
	}

	// nil schema
	err := util.Errors(validateLeafList(nil, &struct{}{})).Error()
	wantErr := `list schema is nil`
	if got, want := err, wantErr; got != want {
		t.Errorf("nil schema: Unmarshal got error: %v, want error: %v", got, want)
	}

	// bad value type
	err = util.Errors(validateLeafList(validLeafListSchema, struct{}{})).Error()
	wantErr = `expected slice type for valid-leaf-list-schema, got struct {}`
	if got, want := err, wantErr; got != want {
		t.Errorf("nil schema: Unmarshal got error: %v, want error: %v", got, want)
	}
}

type LeafListContainer struct {
	Int32LeafList  []*int32        `path:"int32-leaf-list"`
	EnumLeafList   []EnumType      `path:"enum-leaf-list"`
	UnionLeafSlice []UnionLeafType `path:"union-leaflist"`
}

func (*LeafListContainer) ΛEnumTypeMap() map[string][]reflect.Type {
	return map[string][]reflect.Type{
		"/container-schema/union-leaflist": {reflect.TypeOf(EnumType(0)), reflect.TypeOf(EnumType2(0))},
	}
}

func (*LeafListContainer) To_UnionLeafType(i interface{}) (UnionLeafType, error) {
	switch v := i.(type) {
	case string:
		return &UnionLeafType_String{v}, nil
	case uint32:
		return &UnionLeafType_Uint32{v}, nil
	case EnumType:
		return &UnionLeafType_EnumType{v}, nil
	case EnumType2:
		return &UnionLeafType_EnumType2{v}, nil
	default:
		return nil, fmt.Errorf("cannot convert %v to To_UnionLeafType, unknown union type, got: %T, want any of [string, uint32]", i, i)
	}
}

func TestUnmarshalLeafListGNMIEncoding(t *testing.T) {
	containerSchema := &yang.Entry{
		Name: "container-schema",
		Kind: yang.DirectoryEntry,
		Dir: map[string]*yang.Entry{
			"leaf": {
				Name: "leaf",
				Kind: yang.LeafEntry,
				Type: &yang.YangType{
					Kind:    yang.Ystring,
					Pattern: []string{"b+"},
				},
			},
		},
	}
	int32LeafListSchema := &yang.Entry{
		Parent:   containerSchema,
		Name:     "int32-leaf-list",
		Kind:     yang.LeafEntry,
		ListAttr: &yang.ListAttr{MinElements: &yang.Value{Name: "0"}},
		Type:     &yang.YangType{Kind: yang.Yint32},
	}

	enumLeafListSchema := &yang.Entry{
		Parent:   containerSchema,
		Name:     "enum-leaf-list",
		Kind:     yang.LeafEntry,
		ListAttr: &yang.ListAttr{MinElements: &yang.Value{Name: "0"}},
		Type:     &yang.YangType{Kind: yang.Yenum},
	}

	unionLeafListSchema := &yang.Entry{
		Parent:   containerSchema,
		Name:     "union-leaflist",
		Kind:     yang.LeafEntry,
		ListAttr: &yang.ListAttr{MinElements: &yang.Value{Name: "0"}},
		Type: &yang.YangType{
			Kind: yang.Yunion,
			Type: []*yang.YangType{
				{
					Kind:    yang.Ystring,
					Pattern: []string{"a+"},
				},
				{
					Kind: yang.Yuint32,
				},
				{
					Kind: yang.Yenum,
				},
			},
		},
	}

	tests := []struct {
		desc    string
		sch     *yang.Entry
		val     interface{}
		want    LeafListContainer
		wantErr string
	}{
		{
			desc: "nil success",
			want: LeafListContainer{},
		},
		{
			desc: "int32 success",
			sch:  int32LeafListSchema,
			val: &gpb.TypedValue{Value: &gpb.TypedValue_LeaflistVal{
				LeaflistVal: &gpb.ScalarArray{
					Element: []*gpb.TypedValue{
						{Value: &gpb.TypedValue_IntVal{IntVal: -42}},
						{Value: &gpb.TypedValue_IntVal{IntVal: 0}},
						{Value: &gpb.TypedValue_IntVal{IntVal: 42}},
					},
				},
			}},
			want: LeafListContainer{Int32LeafList: []*int32{ygot.Int32(-42), ygot.Int32(0), ygot.Int32(42)}},
		},
		{
			desc: "enum success",
			sch:  enumLeafListSchema,
			val: &gpb.TypedValue{Value: &gpb.TypedValue_LeaflistVal{
				LeaflistVal: &gpb.ScalarArray{
					Element: []*gpb.TypedValue{
						{Value: &gpb.TypedValue_StringVal{StringVal: "E_VALUE_FORTY_TWO"}},
					},
				},
			}},
			want: LeafListContainer{EnumLeafList: []EnumType{42}},
		},
		{
			desc: "unionleaf success",
			sch:  unionLeafListSchema,
			val: &gpb.TypedValue{Value: &gpb.TypedValue_LeaflistVal{
				LeaflistVal: &gpb.ScalarArray{
					Element: []*gpb.TypedValue{
						{Value: &gpb.TypedValue_StringVal{StringVal: "forty two"}},
						{Value: &gpb.TypedValue_StringVal{StringVal: "E_VALUE_FORTY_TWO"}},
						{Value: &gpb.TypedValue_UintVal{UintVal: 42}},
					},
				},
			}},
			want: LeafListContainer{UnionLeafSlice: []UnionLeafType{
				&UnionLeafType_String{String: "forty two"},
				&UnionLeafType_EnumType{EnumType: EnumType(42)},
				&UnionLeafType_Uint32{Uint32: 42},
			}},
		},
		{
			desc: "fail unionleaf no suitable type",
			sch:  unionLeafListSchema,
			val: &gpb.TypedValue{Value: &gpb.TypedValue_LeaflistVal{
				LeaflistVal: &gpb.ScalarArray{
					Element: []*gpb.TypedValue{
						{Value: &gpb.TypedValue_IntVal{IntVal: 42}},
					},
				},
			}},
			wantErr: "could not find suitable union type to unmarshal value int_val:42",
		},
		{
			desc: "bad array element",
			sch:  int32LeafListSchema,
			val: &gpb.TypedValue{Value: &gpb.TypedValue_LeaflistVal{
				LeaflistVal: &gpb.ScalarArray{
					Element: []*gpb.TypedValue{
						{Value: &gpb.TypedValue_IntVal{IntVal: 42}},
						{Value: &gpb.TypedValue_IntVal{IntVal: 4294967296}},
					},
				},
			}},
			wantErr: `unable to convert "4294967296" to int32`,
		},
		{
			desc: "bad value type",
			sch:  enumLeafListSchema,
			val: &gpb.TypedValue{Value: &gpb.TypedValue_LeaflistVal{
				LeaflistVal: &gpb.ScalarArray{
					Element: []*gpb.TypedValue{
						{Value: &gpb.TypedValue_IntVal{IntVal: 42}},
					},
				},
			}},
			wantErr: "TypedValue_IntVal cannot be unmarshalled into enumeration, value is &{42}",
		},
	}

	for _, tt := range tests {
		var parent LeafListContainer
		err := unmarshalGeneric(tt.sch, &parent, tt.val, GNMIEncoding)
		if diff := errdiff.Substring(err, tt.wantErr); diff != "" {
			t.Errorf("%s: unmarshalGeneric(%v, %v, %v): diff(-got,+want):\n%s", tt.desc, tt.sch, parent, tt.val, diff)
		}
		if err != nil {
			continue
		}
		if got, want := parent, tt.want; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: unmarshalGeneric got:\n%v\nwant:\n%v\n", tt.desc, pretty.Sprint(got), pretty.Sprint(want))
		}
	}
}

func TestUnmarshalLeafListJSONEncoding(t *testing.T) {
	containerWithLeafListSchema := &yang.Entry{
		Name: "container",
		Kind: yang.DirectoryEntry,
		Dir: map[string]*yang.Entry{
			"int32-leaf-list": {
				Name:     "int32-leaf-list",
				Kind:     yang.LeafEntry,
				ListAttr: &yang.ListAttr{MinElements: &yang.Value{Name: "0"}},
				Type:     &yang.YangType{Kind: yang.Yint32},
			},
			"enum-leaf-list": {
				Name:     "enum-leaf-list",
				Kind:     yang.LeafEntry,
				ListAttr: &yang.ListAttr{MinElements: &yang.Value{Name: "0"}},
				Type:     &yang.YangType{Kind: yang.Yenum},
			},
		},
	}
	type ContainerStruct struct {
		Int32LeafList []*int32   `path:"int32-leaf-list"`
		EnumLeafList  []EnumType `path:"enum-leaf-list"`
	}

	tests := []struct {
		desc    string
		json    string
		want    ContainerStruct
		wantErr string
	}{
		{
			desc: "nil success",
			json: ``,
			want: ContainerStruct{},
		},
		{
			desc: "int32 success",
			json: `{ "int32-leaf-list" : [-42, 0, 42] }`,
			want: ContainerStruct{Int32LeafList: []*int32{ygot.Int32(-42), ygot.Int32(0), ygot.Int32(42)}},
		},
		{
			desc: "enum success",
			json: `{ "enum-leaf-list" : ["E_VALUE_FORTY_TWO"] }`,
			want: ContainerStruct{EnumLeafList: []EnumType{42}},
		},
		{
			desc:    "bad field name",
			json:    `{ "bad field" : [42] }`,
			wantErr: `parent container container (type *ytypes.ContainerStruct): JSON contains unexpected field bad field`,
		},
		{
			desc:    "bad array element",
			json:    `{ "int32-leaf-list" : [42, 4294967296] }`,
			wantErr: `error parsing 4.294967296e+09 for schema int32-leaf-list: value 4294967296 falls outside the int range [-2147483648, 2147483647]`,
		},
	}

	var jsonTree interface{}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var parent ContainerStruct

			if tt.json != "" {
				if err := json.Unmarshal([]byte(tt.json), &jsonTree); err != nil {
					t.Fatal(fmt.Sprintf("%s : %s", tt.desc, err))
				}
			}

			err := Unmarshal(containerWithLeafListSchema, &parent, jsonTree)
			if got, want := errToString(err), tt.wantErr; got != want {
				t.Errorf("%s: Unmarshal got error: %v, want error: %v", tt.desc, got, want)
			}
			testErrLog(t, tt.desc, err)
			if err == nil {
				if got, want := parent, tt.want; !reflect.DeepEqual(got, want) {
					t.Errorf("%s: Unmarshal got:\n%v\nwant:\n%v\n", tt.desc, pretty.Sprint(got), pretty.Sprint(want))
				}
			}
		})
	}

	var parent ContainerStruct
	badJSONTree := map[string]interface{}{
		"int32-leaf-list": map[string]interface{}{},
	}

	wantErrStr := `unmarshalLeafList for schema int32-leaf-list: value map[] (map): got type map[string]interface {}, expect []interface{}`
	if got, want := errToString(Unmarshal(containerWithLeafListSchema, &parent, badJSONTree)), wantErrStr; got != want {
		t.Errorf("Unmarshal leaf-list with bad json : got error: %s, want error: %s", got, want)
	}

	// nil value
	if got := unmarshalLeafList(nil, nil, nil, JSONEncoding); got != nil {
		t.Errorf("nil value: Unmarshal got error: %v, want error: nil", got)
	}

	// nil schema
	wantErr := `list schema is nil`
	if got, want := errToString(unmarshalLeafList(nil, nil, []struct{}{}, JSONEncoding)), wantErr; got != want {
		t.Errorf("nil schema: Unmarshal got error: %v, want error: %v", got, want)
	}

	// bad value type
	wantErr = `unmarshalLeafList for schema valid-leaf-list-schema: value 42 (int): got type int, expect []interface{}`
	if got, want := errToString(unmarshalLeafList(validLeafListSchema, &struct{}{}, int(42), JSONEncoding)), wantErr; got != want {
		t.Errorf("nil schema: Unmarshal got error: %v, want error: %v", got, want)
	}
}

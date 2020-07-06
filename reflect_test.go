package jsonschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/jsonschema/fixtures/quicktype"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
)

type GrandfatherType struct {
	FamilyName string `json:"family_name" jsonschema:"required"`
}

type SomeBaseType struct {
	SomeBaseProperty     int `json:"some_base_property"`
	SomeBasePropertyYaml int `yaml:"some_base_property_yaml"`
	// The jsonschema required tag is nonsensical for private and ignored properties.
	// Their presence here tests that the fields *will not* be required in the output
	// schema, even if they are tagged required.
	somePrivateBaseProperty   string          `json:"i_am_private" jsonschema:"required"`
	SomeIgnoredBaseProperty   string          `json:"-" jsonschema:"required"`
	SomeSchemaIgnoredProperty string          `jsonschema:"-,required"`
	Grandfather               GrandfatherType `json:"grand"`

	SomeUntaggedBaseProperty           bool `jsonschema:"required"`
	someUnexportedUntaggedBaseProperty bool
}

type MapType map[string]interface{}

type nonExported struct {
	PublicNonExported  int
	privateNonExported int
}

type ProtoEnum int32

func (ProtoEnum) EnumDescriptor() ([]byte, []int) { return []byte(nil), []int{0} }

const (
	Unset ProtoEnum = iota
	Great
)

type TestUser struct {
	SomeBaseType
	nonExported
	MapType

	ID      int                    `json:"id" jsonschema:"required"`
	Name    string                 `json:"name" jsonschema:"required,minLength=1,maxLength=20,pattern=.*,description=this is a property,title=the name,example=joe,example=lucy,default=alex"`
	Friends []int                  `json:"friends,omitempty" jsonschema_description:"list of IDs, omitted when empty"`
	Tags    map[string]interface{} `json:"tags,omitempty"`

	TestFlag       bool
	IgnoredCounter int `json:"-"`

	// Tests for RFC draft-wright-json-schema-validation-00, section 7.3
	BirthDate time.Time `json:"birth_date,omitempty"`
	Website   url.URL   `json:"website,omitempty"`
	IPAddress net.IP    `json:"network_address,omitempty"`

	// Tests for RFC draft-wright-json-schema-hyperschema-00, section 4
	Photo []byte `json:"photo,omitempty" jsonschema:"required"`

	// Tests for jsonpb enum support
	Feeling ProtoEnum `json:"feeling,omitempty"`
	Age     int       `json:"age" jsonschema:"minimum=18,maximum=120,exclusiveMaximum=true,exclusiveMinimum=true"`
	Email   string    `json:"email" jsonschema:"format=email"`

	// Test for "extras" support
	Baz string `jsonschema_extras:"foo=bar,hello=world,foo=bar1"`

	// Tests for simple enum tags
	Color      string  `json:"color" jsonschema:"enum=red,enum=green,enum=blue"`
	Rank       int     `json:"rank,omitempty" jsonschema:"enum=1,enum=2,enum=3"`
	Multiplier float64 `json:"mult,omitempty" jsonschema:"enum=1.0,enum=1.5,enum=2.0"`
}

type CustomTime time.Time

type CustomTypeField struct {
	CreatedAt CustomTime
}

type RootOneOf struct {
	Field1 string      `json:"field1" jsonschema:"oneof_required=group1"`
	Field2 string      `json:"field2" jsonschema:"oneof_required=group2"`
	Field3 interface{} `json:"field3" jsonschema:"oneof_type=string;array"`
	Field4 string      `json:"field4" jsonschema:"oneof_required=group1"`
	Field5 ChildOneOf  `json:"child"`
}

type ChildOneOf struct {
	Child1 string      `json:"child1" jsonschema:"oneof_required=group1"`
	Child2 string      `json:"child2" jsonschema:"oneof_required=group2"`
	Child3 interface{} `json:"child3" jsonschema:"oneof_required=group2,oneof_type=string;array"`
	Child4 string      `json:"child4" jsonschema:"oneof_required=group1"`
}

type Thing3 struct {
	On    Thing1 `jsonschema:"oneof_type=on"`
	State Thing2 `jsonschema:"oneof_type=state"`
}

type Thing1 bool
type Thing2 struct {
	Start int
	End   int
}

func TestThing3(t *testing.T) {
	tests := []struct {
		typ       interface{}
		reflector *Reflector
		fixture   string
	}{
		{&Thing3{}, &Reflector{RequiredFromJSONSchemaTags: true}, "fixtures/thing.json"},
	}

	for _, tt := range tests {
		name := strings.TrimSuffix(filepath.Base(tt.fixture), ".json")
		t.Run(name, func(t *testing.T) {

			expectedSchema := tt.reflector.Reflect(tt.typ)

			expectedJSON, _ := json.MarshalIndent(expectedSchema, "", "  ")

			// Write file, because verbosity is awesome.
			err := ioutil.WriteFile(tt.fixture, expectedJSON, os.ModePerm)
			if err != nil {
				t.Fatal(err)
			}

			quicktypeSchema := quicktype.Thing3{}
			quicktyped := tt.reflector.Reflect(quicktypeSchema)

			// Write file, because verbosity is awesome.
			quicktypeJSON, _ := json.MarshalIndent(quicktyped, "", "    ")
			err = ioutil.WriteFile(fmt.Sprintf("fixtures/%s_quicktype.json", name), quicktypeJSON, os.ModePerm)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(expectedJSON, quicktypeJSON) {
				t.Logf("\nours=%s\ntheirs=%s\nstruct=%s",
					string(expectedJSON),
					string(quicktypeJSON),
					spew.Sdump(Thing3{}),
				)
				t.Error("quicktype != ours")
			}
		})
	}
}

func TestSchemaGeneration(t *testing.T) {
	tests := []struct {
		typ       interface{}
		reflector *Reflector
		fixture   string
	}{
		{&RootOneOf{}, &Reflector{RequiredFromJSONSchemaTags: true}, "fixtures/oneof.json"},
		{&TestUser{}, &Reflector{}, "fixtures/defaults.json"},
		{&TestUser{}, &Reflector{AllowAdditionalProperties: true}, "fixtures/allow_additional_props.json"},
		{&TestUser{}, &Reflector{RequiredFromJSONSchemaTags: true}, "fixtures/required_from_jsontags.json"},
		{&TestUser{}, &Reflector{ExpandedStruct: true}, "fixtures/defaults_expanded_toplevel.json"},
		{&TestUser{}, &Reflector{IgnoredTypes: []interface{}{GrandfatherType{}}}, "fixtures/ignore_type.json"},
		{&TestUser{}, &Reflector{DoNotReference: true}, "fixtures/no_reference.json"},
		{&TestUser{}, &Reflector{FullyQualifyTypeNames: true}, "fixtures/fully_qualified.json"},
		{&TestUser{}, &Reflector{DoNotReference: true, FullyQualifyTypeNames: true}, "fixtures/no_ref_qual_types.json"},
		{&CustomTypeField{}, &Reflector{
			TypeMapper: func(i reflect.Type) *Type {
				if i == reflect.TypeOf(CustomTime{}) {
					return &Type{
						Type:   "string",
						Format: "date-time",
					}
				}
				return nil
			},
		}, "fixtures/custom_type.json"},
	}

	for _, tt := range tests {
		name := strings.TrimSuffix(filepath.Base(tt.fixture), ".json")
		t.Run(name, func(t *testing.T) {
			f, err := ioutil.ReadFile(tt.fixture)
			require.NoError(t, err)

			actualSchema := tt.reflector.Reflect(tt.typ)
			expectedSchema := &Schema{}

			err = json.Unmarshal(f, expectedSchema)
			require.NoError(t, err)

			expectedJSON, _ := json.MarshalIndent(expectedSchema, "", "  ")
			actualJSON, _ := json.MarshalIndent(actualSchema, "", "  ")
			require.Equal(t, string(expectedJSON), string(actualJSON))
		})
	}
}

func TestBaselineUnmarshal(t *testing.T) {
	expectedJSON, err := ioutil.ReadFile("fixtures/defaults.json")
	require.NoError(t, err)

	reflector := &Reflector{}
	actualSchema := reflector.Reflect(&TestUser{})

	actualJSON, _ := json.MarshalIndent(actualSchema, "", "  ")

	require.Equal(t, strings.Replace(string(expectedJSON), `\/`, "/", -1), string(actualJSON))
}

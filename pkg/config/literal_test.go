package config

import (
	"reflect"
	"testing"

	"github.com/miekg/dns"
)

type testRecord struct {
	Int     int              `record:"int"`
	Int8    int8             `record:"int8"`
	Int16   int16            `record:"int16"`
	Int32   int32            `record:"int32"`
	Int64   int64            `record:"int64"`
	Uint    uint             `record:"uint"`
	Uint8   uint8            `record:"uint8"`
	Uint16  uint16           `record:"uint16"`
	Uint32  uint32           `record:"uint32"`
	Uint64  uint64           `record:"uint64"`
	Uintptr uintptr          `record:"uintptr"`
	String  string           `record:"string"`
	Struct  testStructRecord `record:"struct"`
	Array   [2]int           `record:"array"`
	Slice   []int            `record:"slice"`
	Map     map[string]int   `record:"map"`
}

// conform to dnsconfig.Record
func (tr *testRecord) Convert(name string) []dns.RR {
	return []dns.RR{}
}

type testStructRecord struct {
	Int int `record:"int"`
}

func TestLiteral(t *testing.T) {
	record := &Record{
		Type: "test",
		Name: "test",
		LiteralValue: map[string]any{
			"int":     int(1),
			"int8":    int8(1),
			"int16":   int16(1),
			"int32":   int32(1),
			"int64":   int64(1),
			"uint":    uint(1),
			"uint8":   uint8(1),
			"uint16":  uint16(1),
			"uint32":  uint32(1),
			"uint64":  uint64(1),
			"uintptr": uintptr(1),
			"string":  "a string",
			"struct": map[string]any{
				"int": int(1),
			},
			"array": [2]any{int(1), int(1)},
			"slice": []any{int(1), int(1), int(1)},
			"map": map[string]any{
				"int": int(1),
			},
		},
		Value: &testRecord{},
	}

	if err := record.parseLiteral(); err != nil {
		t.Fatal(err)
	}

	tr, ok := record.Value.(*testRecord)
	if !ok {
		t.Fatalf("record.Value was not *testRecord, was %T", record.Value)
	}

	if tr.Int != int(1) {
		t.Fatalf("int was not set properly")
	}

	if tr.Int8 != int8(1) {
		t.Fatalf("int8 was not set properly")
	}

	if tr.Int16 != int16(1) {
		t.Fatalf("int16 was not set properly")
	}

	if tr.Int32 != int32(1) {
		t.Fatalf("int32 was not set properly")
	}

	if tr.Int64 != int64(1) {
		t.Fatalf("int64 was not set properly")
	}

	if tr.Uint != uint(1) {
		t.Fatalf("uint was not set properly")
	}

	if tr.Uint8 != uint8(1) {
		t.Fatalf("uint8 was not set properly")
	}

	if tr.Uint16 != uint16(1) {
		t.Fatalf("uint16 was not set properly")
	}

	if tr.Uint32 != uint32(1) {
		t.Fatalf("uint32 was not set properly")
	}

	if tr.Uint64 != uint64(1) {
		t.Fatalf("uint64 was not set properly")
	}

	if tr.Uintptr != uintptr(1) {
		t.Fatalf("uintptr was not set properly")
	}

	if tr.String != "a string" {
		t.Fatalf("string was not set properly")
	}

	if !reflect.DeepEqual(tr.Struct, testStructRecord{Int: 1}) {
		t.Fatalf("struct was not set properly: %v", tr.Struct)
	}

	if !reflect.DeepEqual(tr.Array, [2]int{1, 1}) {
		t.Fatalf("array was not set properly: %v", tr.Array)
	}

	if !reflect.DeepEqual(tr.Slice, []int{1, 1, 1}) {
		t.Fatalf("slice was not set properly: %v", tr.Slice)
	}

	if !reflect.DeepEqual(tr.Map, map[string]int{"int": 1}) {
		t.Fatalf("map was not set properly: %v", tr.Map)
	}
}

package config

import (
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
			"map": map[any]any{
				"int": int(1),
			},
		},
		Value: &testRecord{},
	}

	if err := record.parseLiteral(); err != nil {
		t.Fatal(err)
	}
}

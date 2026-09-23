package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestAppendToJSONArrayKeepsExistingFormatting(t *testing.T) {
	existing, err := os.ReadFile("../../garoop-data/public/novel/novels.json")
	if err != nil {
		t.Skip("garoop-data が隣に無いためスキップ")
	}
	out, err := appendToJSONArray(existing, map[string]string{"id": "new"})
	if err != nil {
		t.Fatal(err)
	}
	trimmed := bytes.TrimSuffix(bytes.TrimRight(existing, "\n"), []byte("]"))
	if !bytes.HasPrefix(out, bytes.TrimRight(trimmed, "\n ")) {
		t.Fatalf("既存部分の体裁が変わっています")
	}
	if !strings.HasSuffix(string(out), "  {\n    \"id\": \"new\"\n  }\n]\n") {
		t.Fatalf("追加部分の体裁が想定外: %q", string(out[len(out)-60:]))
	}
}

func TestAppendToJSONArrayEmpty(t *testing.T) {
	out, err := appendToJSONArray(nil, map[string]int{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "[\n  {\n    \"a\": 1\n  }\n]\n" {
		t.Fatalf("unexpected: %q", out)
	}
}

func TestTVInRangeWrapsMidnight(t *testing.T) {
	if !tvInRange(23*60, "22:00", "05:59") || !tvInRange(3*60, "22:00", "05:59") || tvInRange(12*60, "22:00", "05:59") {
		t.Fatal("日付またぎの判定が誤っています")
	}
}

func TestContainsProfile(t *testing.T) {
	if !containsProfile("garoop,garooptv", "garooptv") || containsProfile("garoop", "garuchan") {
		t.Fatal("profile 判定が誤っています")
	}
}

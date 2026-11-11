package vimeo

import (
	"encoding/json"
	"testing"
)

func TestThumbnail(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"no pictures", `{}`, ""},
		{"gray placeholder", `{"pictures":{"default_picture":true,"sizes":[{"width":640,"link":"p"}]}}`, ""},
		{"smallest at least 640 wide", `{"pictures":{"sizes":[{"width":100,"link":"a"},{"width":1920,"link":"b"},{"width":640,"link":"c"},{"width":960,"link":"d"}]}}`, "c"},
		{"largest when all are small", `{"pictures":{"sizes":[{"width":100,"link":"a"},{"width":295,"link":"b"},{"width":200,"link":"c"}]}}`, "b"},
	} {
		var v Video
		if err := json.Unmarshal([]byte(tc.body), &v); err != nil {
			t.Fatal(err)
		}
		if got := v.Thumbnail(); got != tc.want {
			t.Errorf("%s: Thumbnail() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

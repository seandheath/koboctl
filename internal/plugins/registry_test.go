package plugins

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantVer  string
	}{
		{"dynamic_panelzoom", "dynamic_panelzoom", ""},
		{"dynamic_panelzoom@v1.7.0", "dynamic_panelzoom", "v1.7.0"},
		{"foo@latest", "foo", "latest"},
		{"", "", ""},
	}
	for _, c := range cases {
		name, ver := Parse(c.in)
		if name != c.wantName || ver != c.wantVer {
			t.Errorf("Parse(%q) = (%q, %q), want (%q, %q)", c.in, name, ver, c.wantName, c.wantVer)
		}
	}
}

func TestLookup(t *testing.T) {
	if _, ok := Lookup("dynamic_panelzoom"); !ok {
		t.Error("dynamic_panelzoom should be a registered plugin")
	}
	src, _ := Lookup("dynamic_panelzoom")
	if src.Owner != "JorgeTheFox" || src.Repo != "koreader-dynamic-panelzoom" {
		t.Errorf("unexpected source for dynamic_panelzoom: %+v", src)
	}
	if src.AssetPattern != "dynamic_panelzoom.koplugin.zip" {
		t.Errorf("unexpected asset pattern: %q", src.AssetPattern)
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("unknown plugin should not be found")
	}
}

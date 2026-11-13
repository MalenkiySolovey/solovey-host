package publicsurface

import "testing"

func TestClassifiersDiscardRequestContent(t *testing.T) {
	if ClassifyPath("/secret?token=private", true) != "reserved_panel" || ClassifyPath("/.env?password=private", false) != "scanner_path" || ClassifyUserAgent("curl/secret") != "ua_scanner" {
		t.Fatal("request content classification changed")
	}
	if ClassifyPath(string([]byte{255}), false) != "invalid_utf8" || ClassifyPath("/asset.css", false) != "static_asset" || ClassifyPath("/page", false) != "fallback_path" {
		t.Fatal("path classification changed")
	}
	if ClassifyStatus(503) != "5xx" || ClassifyMethod("HEAD") != "get" || ClassifyBytes(65536) != "large" || ClassifyDuration(999) != "normal" {
		t.Fatal("bounded observations changed")
	}
}

package singbox

import (
	"errors"
	"testing"

	"github.com/liulei/proxymorph/internal/convert"
)

func TestAdapterAvailableReportsMissingBinary(t *testing.T) {
	err := New("/definitely/missing/sing-box").Available()
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestConvertVLESSSimpleTLS(t *testing.T) {
	out, err := New("").ConvertVLESS(convert.Node{
		Name:     "Edge",
		Protocol: "vless",
		Server:   "edge.example.com",
		Port:     443,
		Params: map[string]string{
			"uuid": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			"tls":  "tls",
			"sni":  "sni.example.com",
		},
	})
	if err != nil {
		t.Fatalf("ConvertVLESS returned error: %v", err)
	}
	if out == "" {
		t.Fatal("expected rendered VLESS output")
	}
}

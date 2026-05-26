package singbox

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/liulei/proxymorph/internal/convert"
)

var ErrUnavailable = errors.New("sing-box unavailable")

type Adapter struct {
	Path string
}

func New(path string) *Adapter {
	if path == "" {
		path = "sing-box"
	}
	return &Adapter{Path: path}
}

func (a *Adapter) Available() error {
	if _, err := exec.LookPath(a.Path); err != nil {
		return fmt.Errorf("%w: %s", ErrUnavailable, a.Path)
	}
	return nil
}

func (a *Adapter) ConvertVLESS(node convert.Node) (string, error) {
	if node.Protocol != "vless" {
		return "", fmt.Errorf("sing-box adapter only converts vless nodes")
	}
	if node.Params["uuid"] == "" {
		return "", fmt.Errorf("vless uuid is required")
	}
	return convert.RenderSurge6([]convert.Node{node}, nil, nil), nil
}

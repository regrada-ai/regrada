package git

import (
	"bytes"
	"fmt"
	"os/exec"
)

type Client interface {
	ShowFile(ref, path string) ([]byte, error)
}

type ExecClient struct{}

func NewExecClient() *ExecClient {
	return &ExecClient{}
}

func (c *ExecClient) ShowFile(ref, path string) ([]byte, error) {
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", ref, path))
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git show %s:%s: %v (%s)", ref, path, err, stderr.String())
	}
	return out.Bytes(), nil
}

package git

import (
	"fmt"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Client interface {
	ShowFile(ref, path string) ([]byte, error)
	GetCurrentCommit() (string, error)
	GetCurrentBranch() (string, error)
	GetCommitMessage(sha string) (string, error)
}

type ExecClient struct {
	repo *gogit.Repository
}

func NewExecClient() *ExecClient {
	// Try to open the repository from current directory
	repo, err := gogit.PlainOpenWithOptions(".", &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		// If we can't open repo, return client with nil repo
		// Methods will gracefully handle this
		return &ExecClient{repo: nil}
	}
	return &ExecClient{repo: repo}
}

func (c *ExecClient) ShowFile(ref, path string) ([]byte, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("repository not available")
	}

	// Resolve the reference to a hash
	hash, err := c.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, fmt.Errorf("resolve ref %s: %w", ref, err)
	}

	// Get the commit
	commit, err := c.repo.CommitObject(*hash)
	if err != nil {
		return nil, fmt.Errorf("get commit %s: %w", hash, err)
	}

	// Get the tree
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	// Get the file from the tree
	file, err := tree.File(path)
	if err != nil {
		return nil, fmt.Errorf("get file %s from tree: %w", path, err)
	}

	// Read file contents
	contents, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("read file contents: %w", err)
	}

	return []byte(contents), nil
}

// GetCurrentCommit returns the current commit SHA
func (c *ExecClient) GetCurrentCommit() (string, error) {
	if c.repo == nil {
		return "", fmt.Errorf("repository not available")
	}

	head, err := c.repo.Head()
	if err != nil {
		return "", err
	}

	return head.Hash().String(), nil
}

// GetCurrentBranch returns the current branch name
func (c *ExecClient) GetCurrentBranch() (string, error) {
	if c.repo == nil {
		return "", fmt.Errorf("repository not available")
	}

	head, err := c.repo.Head()
	if err != nil {
		return "", err
	}

	// Get branch name from reference (e.g., "refs/heads/main" -> "main")
	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}

	return "", fmt.Errorf("HEAD is not a branch")
}

// GetCommitMessage returns the commit message for a given SHA
func (c *ExecClient) GetCommitMessage(sha string) (string, error) {
	if c.repo == nil {
		return "", fmt.Errorf("repository not available")
	}

	hash := plumbing.NewHash(sha)
	commit, err := c.repo.CommitObject(hash)
	if err != nil {
		return "", err
	}

	// Get the first line of the commit message
	lines := strings.Split(commit.Message, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}

	return "", nil
}

// Helper function to get commit object
func (c *ExecClient) getCommit(ref string) (*object.Commit, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("repository not available")
	}

	hash, err := c.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, err
	}

	return c.repo.CommitObject(*hash)
}

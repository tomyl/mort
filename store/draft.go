package store

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	xdg "github.com/queria/golang-go-xdg"
)

// SetDraft saves task body to a file.
func SetDraft(id int64, body string) error {
	filepath, err := getDraftPath(id)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath, []byte(body), os.ModePerm)
}

func getDraftPath(id int64) (string, error) {
	name := "new"
	if id > 0 {
		name = fmt.Sprintf("%d", id)
	}
	return xdg.Data.Ensure("mort/draft/" + name)
}

// GetDraft opens an editor for provided task id. Call SetDraft() first.
func GetDraft(id int64, insert bool) (string, error) {
	filepath, err := getDraftPath(id)
	if err != nil {
		return "", err
	}
	args := make([]string, 0)
	if insert {
		args = append(args, "+startinsert")
	}
	args = append(args, filepath, "+9999")
	cmd := exec.Command(getEditor(), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	buf, err := ioutil.ReadFile(filepath)

	if err != nil {
		return "", err
	}

	return string(buf), nil
}

func getEditor() string {
	editor := os.Getenv("EDITOR")
	if editor != "" {
		return editor
	}
	return "vim"
}

// GetProjectFromTitle extracts the project name from the task title.
func GetProjectFromTitle(body string) string {
	idx := strings.Index(body, ":")

	if idx >= 0 {
		return body[:idx]
	}

	return "default"
}

// GetTitleFromBody extracts the task title from the task body.
func GetTitleFromBody(body string) string {
	idx := strings.Index(body, "\n")

	if idx >= 0 {
		return body[:idx]
	}

	return body
}

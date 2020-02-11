package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/grafana/tanka/pkg/kubernetes/manifest"
)

// DiffName computes the filename for use with `DiffStr`
func DiffName(m manifest.Manifest) string {
	return strings.Replace(fmt.Sprintf("%s.%s.%s.%s",
		m.APIVersion(),
		m.Kind(),
		m.Metadata().Namespace(),
		m.Metadata().Name(),
	), "/", "-", -1)
}

// Diff computes the differences between the strings `is` and `should` using the
// UNIX `diff(1)` utility.
func DiffStr(name, is, should string) (string, error) {
	dir, err := ioutil.TempDir("", "diff")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	if err := ioutil.WriteFile(filepath.Join(dir, "LIVE-"+name), []byte(is), os.ModePerm); err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "MERGED-"+name), []byte(should), os.ModePerm); err != nil {
		return "", err
	}

	buf := bytes.Buffer{}
	merged := filepath.Join(dir, "MERGED-"+name)
	live := filepath.Join(dir, "LIVE-"+name)

	var cmd *exec.Cmd
	if isCommandAvailable("icdiff") {
		cols := strings.Join([]string{"--cols=", terminal_width()}, "")
		cmd = exec.Command("icdiff", "-r", cols, live, merged)
	} else {
		cmd = exec.Command("diff", "-u", "-N", live, merged)
	}

	cmd.Stdout = &buf
	err = cmd.Run()

	// the diff utility exits with `1` if there are differences. We need to not fail there.
	if exitError, ok := err.(*exec.ExitError); ok && err != nil {
		if exitError.ExitCode() != 1 {
			return "", err
		}
	}

	out := buf.String()
	if out != "" {
		out = fmt.Sprintf("%s\n%s", cmd, out)
	}

	return out, nil
}

// Diffstat uses `diffstat(1)` utility to summarize a `diff(1)` output
func Diffstat(d string) (*string, error) {
	cmd := exec.Command("diffstat", "-C")
	buf := bytes.Buffer{}
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	cmd.Stdin = strings.NewReader(d)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("invoking diffstat(1): %s", err.Error())
	}

	out := buf.String()
	return &out, nil
}

// FilteredErr is a filtered Stderr. If one of the regular expressions match, the current input is discarded.
type FilteredErr []*regexp.Regexp

func (r FilteredErr) Write(p []byte) (n int, err error) {
	for _, re := range r {
		if re.Match(p) {
			// silently discard
			return len(p), nil
		}
	}
	return os.Stderr.Write(p)
}

func terminal_width() string {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return "80"
	}
	return strings.Split(string(out), " ")[1]
}

func isCommandAvailable(name string) bool {
	cmd := exec.Command("/bin/sh", "-c", "command -v "+name)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Command not found: %s\n", name)
		return false
	}
	return true
}

/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package commands

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/osscontainertools/kaniko/pkg/dockerfile"
	"github.com/osscontainertools/kaniko/pkg/util"
	"github.com/osscontainertools/kaniko/testutil"
)

func Test_addDefaultHOME(t *testing.T) {
	tests := []struct {
		name        string
		user        string
		mockUser    *user.User
		lookupError error
		initial     []string
		expected    []string
	}{
		{
			name: "HOME already set",
			user: "",
			initial: []string{
				"HOME=/something",
				"PATH=/something/else",
			},
			expected: []string{
				"HOME=/something",
				"PATH=/something/else",
			},
		},
		{
			name: "HOME not set and user not set",
			user: "",
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/root",
			},
		},
		{
			name: "HOME not set and user and homedir for the user set",
			user: "www-add",
			mockUser: &user.User{
				Username: "www-add",
				HomeDir:  "/home/some-other",
			},
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/home/some-other",
			},
		},
		{
			name: "USER is set using the UID",
			user: "1000",
			mockUser: &user.User{
				Username: "1000",
				HomeDir:  "/",
			},
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/",
			},
		},
		{
			name: "HOME not set and user is set to root",
			user: "root",
			mockUser: &user.User{
				Username: "root",
			},
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/root",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := userLookup
			userLookup = func(_ string) (*user.User, error) { return test.mockUser, test.lookupError }
			defer func() {
				userLookup = original
			}()
			actual, err := addDefaultHOME(test.user, test.initial)
			testutil.CheckErrorAndDeepEqual(t, false, err, test.expected, actual)
		})
	}
}

func TestSetWorkDirIfExists(t *testing.T) {
	testDir := t.TempDir()
	testutil.CheckDeepEqual(t, testDir, setWorkDirIfExists(testDir))
	testutil.CheckDeepEqual(t, "", setWorkDirIfExists("doesnot-exists"))
}

// Scenario 1: Pre-existing /a/b, tmpfs target is /a/b/c.
// Files moved to /a and copied to /a/b during RUN exist after the RUN step.
func TestRunCommandWithFlags_TmpfsMount_Scenario1(t *testing.T) {
	tmpDir := t.TempDir()
	dirA := filepath.Join(tmpDir, "a")
	dirB := filepath.Join(dirA, "b")
	targetC := filepath.Join(dirB, "c")

	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatalf("failed to create /a/b: %v", err)
	}

	cmdStr := "mkdir -p " + targetC + " && echo 'moved' > " + filepath.Join(targetC, "movefile") +
		" && echo 'copied' > " + filepath.Join(targetC, "copyfile") +
		" && mv " + filepath.Join(targetC, "movefile") + " " + filepath.Join(dirA, "movefile") +
		" && cp " + filepath.Join(targetC, "copyfile") + " " + filepath.Join(dirB, "copyfile")

	dockerfileSnippet := "RUN --mount=type=tmpfs,target=" + targetC + " " + cmdStr
	res, err := parser.Parse(strings.NewReader(dockerfileSnippet))
	if err != nil {
		t.Fatalf("failed to parse Dockerfile snippet: %v", err)
	}
	inst, err := instructions.ParseInstruction(res.AST.Children[0])
	if err != nil {
		t.Fatalf("failed to parse instruction: %v", err)
	}
	cmdRun := inst.(*instructions.RunCommand)

	cfg := &v1.Config{}
	buildArgs := dockerfile.NewBuildArgs([]string{})
	fileContext := util.FileContext{Root: tmpDir}

	if err := runCommandWithFlags(cfg, buildArgs, cmdRun, fileContext, nil); err != nil {
		t.Fatalf("runCommandWithFlags returned error: %v", err)
	}

	// Verify moved file exists in /a
	movedContent, err := os.ReadFile(filepath.Join(dirA, "movefile"))
	if err != nil {
		t.Fatalf("failed to read moved file in /a: %v", err)
	}
	if string(movedContent) != "moved\n" {
		t.Errorf("expected 'moved\\n', got %q", string(movedContent))
	}

	// Verify copied file exists in /a/b
	copiedContent, err := os.ReadFile(filepath.Join(dirB, "copyfile"))
	if err != nil {
		t.Fatalf("failed to read copied file in /a/b: %v", err)
	}
	if string(copiedContent) != "copied\n" {
		t.Errorf("expected 'copied\\n', got %q", string(copiedContent))
	}

	// Verify tmpfs mount target /a/b/c was cleaned up
	if _, err := os.Stat(targetC); !os.IsNotExist(err) {
		t.Errorf("expected tmpfs target /a/b/c to be cleaned up after RUN, but it exists")
	}
}

// Scenario 2: /a/b does NOT exist, tmpfs target is /a/b/c.
// Files and implicitly created parent directories created inside tmpfs do not exist afterwards.
func TestRunCommandWithFlags_TmpfsMount_Scenario2(t *testing.T) {
	tmpDir := t.TempDir()
	dirA := filepath.Join(tmpDir, "a")
	dirB := filepath.Join(dirA, "b")
	targetC := filepath.Join(dirB, "c")

	cmdStr := "echo 'tmp' > " + filepath.Join(targetC, "tempfile")

	dockerfileSnippet := "RUN --mount=type=tmpfs,target=" + targetC + " " + cmdStr
	res, err := parser.Parse(strings.NewReader(dockerfileSnippet))
	if err != nil {
		t.Fatalf("failed to parse Dockerfile snippet: %v", err)
	}
	inst, err := instructions.ParseInstruction(res.AST.Children[0])
	if err != nil {
		t.Fatalf("failed to parse instruction: %v", err)
	}
	cmdRun := inst.(*instructions.RunCommand)

	cfg := &v1.Config{}
	buildArgs := dockerfile.NewBuildArgs([]string{})
	fileContext := util.FileContext{Root: tmpDir}

	if err := runCommandWithFlags(cfg, buildArgs, cmdRun, fileContext, nil); err != nil {
		t.Fatalf("runCommandWithFlags returned error: %v", err)
	}

	// Verify targetC /a/b/c was cleaned up after RUN step
	if _, err := os.Stat(targetC); !os.IsNotExist(err) {
		t.Errorf("expected target /a/b/c to be cleaned up after RUN, but it exists")
	}
}

// Scenario 3: Pre-existing /a, tmpfs target is /a.
// Pre-existing contents of /a remain unchanged, while new contents written inside /a during RUN do not exist afterwards.
func TestRunCommandWithFlags_TmpfsMount_Scenario3(t *testing.T) {
	tmpDir := t.TempDir()
	dirA := filepath.Join(tmpDir, "a")

	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatalf("failed to create /a: %v", err)
	}
	preExistingFile := filepath.Join(dirA, "pre_existing.txt")
	if err := os.WriteFile(preExistingFile, []byte("pre-existing content"), 0o644); err != nil {
		t.Fatalf("failed to create pre-existing file in /a: %v", err)
	}

	cmdStr := "echo 'new tmpfs content' > " + filepath.Join(dirA, "new_file.txt")

	dockerfileSnippet := "RUN --mount=type=tmpfs,target=" + dirA + " " + cmdStr
	res, err := parser.Parse(strings.NewReader(dockerfileSnippet))
	if err != nil {
		t.Fatalf("failed to parse Dockerfile snippet: %v", err)
	}
	inst, err := instructions.ParseInstruction(res.AST.Children[0])
	if err != nil {
		t.Fatalf("failed to parse instruction: %v", err)
	}
	cmdRun := inst.(*instructions.RunCommand)

	cfg := &v1.Config{}
	buildArgs := dockerfile.NewBuildArgs([]string{})
	fileContext := util.FileContext{Root: tmpDir}

	if err := runCommandWithFlags(cfg, buildArgs, cmdRun, fileContext, nil); err != nil {
		t.Fatalf("runCommandWithFlags returned error: %v", err)
	}

	// Verify pre-existing file content remains unchanged
	content, err := os.ReadFile(preExistingFile)
	if err != nil {
		t.Fatalf("failed to read pre-existing file: %v", err)
	}
	if string(content) != "pre-existing content" {
		t.Errorf("expected 'pre-existing content', got %q", string(content))
	}

	// Verify new file created inside /a during RUN step does not exist afterwards
	newFile := filepath.Join(dirA, "new_file.txt")
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Errorf("expected new file created inside tmpfs mount /a to be discarded, but it exists")
	}
}

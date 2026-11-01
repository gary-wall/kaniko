/*
Copyright 2026 Google LLC

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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	kConfig "github.com/osscontainertools/kaniko/pkg/config"
	"github.com/osscontainertools/kaniko/pkg/dockerfile"
	"github.com/osscontainertools/kaniko/pkg/util"
	"github.com/osscontainertools/kaniko/testutil"
)

func Test_AddURL_DestSymlinkDir(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "test content from url")
	}))
	defer server.Close()

	testDir := t.TempDir()

	realDir := filepath.Join(testDir, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(testDir, "linkdir")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}

	url := server.URL + "/testfile.txt"

	// Test with FF_KANIKO_ADD_FOLLOW_SYMLINK = false (default)
	t.Run("feature flag disabled fails with symlink error", func(t *testing.T) {
		kConfig.FF.AddFollowSymlink = false
		defer func() {
			kConfig.InitFeatureFlags()
		}()

		cmd := AddCommand{
			cmd: &instructions.AddCommand{
				SourcesAndDest: instructions.SourcesAndDest{
					SourcePaths: []string{url},
					DestPath:    linkDir + "/",
				},
			},
			fileContext: util.FileContext{Root: testDir},
		}

		cfg := &v1.Config{
			WorkingDir: testDir,
		}

		err := cmd.ExecuteCommand(cfg, dockerfile.NewBuildArgs([]string{}))
		if err == nil {
			t.Fatal("expected error when AddFollowSymlink is false, but got nil")
		}
	})

	// Test with FF_KANIKO_ADD_FOLLOW_SYMLINK = true
	t.Run("feature flag enabled resolves symlink and writes to real target", func(t *testing.T) {
		kConfig.FF.AddFollowSymlink = true
		defer func() {
			kConfig.InitFeatureFlags()
		}()

		cmd := AddCommand{
			cmd: &instructions.AddCommand{
				SourcesAndDest: instructions.SourcesAndDest{
					SourcePaths: []string{url},
					DestPath:    linkDir + "/",
				},
			},
			fileContext: util.FileContext{Root: testDir},
		}

		cfg := &v1.Config{
			WorkingDir: testDir,
		}

		err := cmd.ExecuteCommand(cfg, dockerfile.NewBuildArgs([]string{}))
		testutil.CheckNoError(t, err)

		// Check that the file was written to the real directory
		expectedFile := filepath.Join(realDir, "testfile.txt")
		content, err := os.ReadFile(expectedFile)
		testutil.CheckNoError(t, err)
		testutil.CheckDeepEqual(t, "test content from url", string(content))

		// Check that linkdir is still a symlink
		fi, err := os.Lstat(linkDir)
		testutil.CheckNoError(t, err)
		if !util.IsSymlink(fi) {
			t.Fatalf("expected %s to remain a symlink", linkDir)
		}

		// Check that snapshotFiles contains the resolved file path
		snapshotFiles := cmd.FilesToSnapshot()
		expectedSnapshot := []string{expectedFile}
		testutil.CheckDeepEqual(t, expectedSnapshot, snapshotFiles)
	})
}

package testbakecontextdockerfile

import "github.com/osscontainertools/kaniko/golden/types"

var Tests = types.GoldenTests{
	Name:       "test_bake_context_dockerfile",
	Dockerfile: "Dockerfile",
	Tests: []types.GoldenTest{
		{
			Args: []string{"app"},
			Plan: "app",
			Env:  map[string]string{"FF_KANIKO_BAKE_CONTEXT_DOCKERFILE": "1"},
		},
		{
			Args: []string{"tools"},
			Plan: "tools",
			Env:  map[string]string{"FF_KANIKO_BAKE_CONTEXT_DOCKERFILE": "1"},
		},
		{
			Args: []string{},
			Plan: "all",
			Env:  map[string]string{"FF_KANIKO_BAKE_CONTEXT_DOCKERFILE": "1"},
		},
	},
}

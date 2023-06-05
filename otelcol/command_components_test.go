// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"go.opentelemetry.io/collector/component"
)

func TestNewBuildSubCommand(t *testing.T) {
	factories, err := nopFactories()
	require.NoError(t, err)

	cfgProvider, err := NewConfigProvider(newDefaultConfigProviderSettings([]string{filepath.Join("testdata", "otelcol-nop.yaml")}))
	require.NoError(t, err)

	set := CollectorSettings{
		BuildInfo:      component.NewDefaultBuildInfo(),
		Factories:      factories,
		ConfigProvider: cfgProvider,
	}
	cmd := NewCommand(set)
	cmd.SetArgs([]string{"components"})

	ExpectedYamlStruct := componentsOutput{
		BuildInfo:  component.NewDefaultBuildInfo(),
		Receivers:  []component.Type{"nop"},
		Processors: []component.Type{"nop"},
		Exporters:  []component.Type{"nop"},
		Extensions: []component.Type{"nop"},
	}
	ExpectedOutput, err := yaml.Marshal(ExpectedYamlStruct)
	require.NoError(t, err)

	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	err = cmd.Execute()
	require.NoError(t, err)

	// Trim new line at the end of the two strings to make a better comparison as string() adds an extra new
	// line that makes the test fail.
	assert.Equal(t, strings.Trim(string(ExpectedOutput), "\n"), strings.Trim(b.String(), "\n"))
}

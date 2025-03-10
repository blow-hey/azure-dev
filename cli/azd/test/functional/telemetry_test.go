// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package cli_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/azure/azure-dev/cli/azd/internal/telemetry/fields"
	"github.com/azure/azure-dev/cli/azd/pkg/environment"
	"github.com/azure/azure-dev/cli/azd/pkg/environment/azdcontext"
	"github.com/azure/azure-dev/cli/azd/test/azdcli"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

// Span is the format generated by stdouttrace, which is used by azd when --trace-log-file is specified.
// stdouttrace is not a stable exporter, and thus we have a minimal struct that can be modified when needed.
type Span struct {
	Name       string
	Resource   []Attribute
	Attributes []Attribute
}

type Value struct {
	Type  string
	Value interface{}
}

type Attribute struct {
	Key   string
	Value Value
}

var Sha256Regex = regexp.MustCompile("^[A-Fa-f0-9]{64}$")

// Verifies telemetry usage data generated when environments are created.
func Test_CLI_Telemetry_Usage_Data_EnvCreate(t *testing.T) {
	// CLI process and working directory are isolated
	t.Parallel()
	ctx, cancel := newTestContext(t)
	defer cancel()

	dir := tempDirWithDiagnostics(t)
	t.Logf("DIR: %s", dir)

	cli := azdcli.NewCLI(t)
	// Always set telemetry opt-inn setting to avoid influence from user settings
	cli.Env = append(os.Environ(), "AZURE_DEV_COLLECT_TELEMETRY=yes")
	cli.WorkingDirectory = dir

	envName := randomEnvName()

	err := copySample(dir, "storage")
	require.NoError(t, err, "failed expanding sample")

	traceFilePath := filepath.Join(dir, "trace.json")

	envNew(ctx, t, cli, envName, true, "--trace-log-file", traceFilePath)

	traceContent, err := os.ReadFile(traceFilePath)
	require.NoError(t, err)

	scanner := bufio.NewScanner(bytes.NewReader(traceContent))
	usageCmdFound := false
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}

		var span Span
		err = json.Unmarshal(scanner.Bytes(), &span)
		require.NoError(t, err)

		verifyResource(t, span.Resource)
		if strings.HasPrefix(span.Name, "cmd.") {
			usageCmdFound = true
			m := attributesMap(span.Attributes)
			require.Contains(t, m, fields.SubscriptionIdKey)
			require.Equal(t, m[fields.SubscriptionIdKey], getEnvSubscriptionId(t, dir, envName))
		}
	}

	require.True(t, usageCmdFound)
}

// Verifies telemetry usage data generated when environments and projects are loaded.
func Test_CLI_Telemetry_Usage_Data_EnvProjectLoad(t *testing.T) {
	// CLI process and working directory are isolated
	ctx, cancel := newTestContext(t)
	defer cancel()

	dir := tempDirWithDiagnostics(t)
	t.Logf("DIR: %s", dir)

	cli := azdcli.NewCLI(t)
	// Always set telemetry opt-inn setting to avoid influence from user settings
	cli.Env = append(os.Environ(), "AZURE_DEV_COLLECT_TELEMETRY=yes")
	cli.WorkingDirectory = dir

	envName := randomEnvName()

	err := copySample(dir, "restoreapp")
	require.NoError(t, err, "failed expanding sample")

	traceFilePath := filepath.Join(dir, "trace.json")

	_, err = cli.RunCommandWithStdIn(
		ctx,
		stdinForTests(envName),
		"restore", "--service", "csharpapptest",
		"--trace-log-file", traceFilePath)
	require.NoError(t, err)

	traceContent, err := os.ReadFile(traceFilePath)
	require.NoError(t, err)

	scanner := bufio.NewScanner(bytes.NewReader(traceContent))
	usageCmdFound := false
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}

		var span Span
		err = json.Unmarshal(scanner.Bytes(), &span)
		require.NoError(t, err)

		verifyResource(t, span.Resource)
		if span.Name == "cmd.restore" {
			usageCmdFound = true
			m := attributesMap(span.Attributes)
			require.Contains(t, m, fields.SubscriptionIdKey)
			require.Equal(t, m[fields.SubscriptionIdKey], getEnvSubscriptionId(t, dir, envName))

			require.Contains(t, m, fields.TemplateIdKey)
			require.Equal(t, m[fields.TemplateIdKey], fields.CaseInsensitiveHash("azd-test/restoreapp@v1"))

		}
	}
	require.True(t, usageCmdFound)
}

func attributesMap(attributes []Attribute) map[attribute.Key]interface{} {
	m := map[attribute.Key]interface{}{}
	for _, attrib := range attributes {
		m[attribute.Key(attrib.Key)] = attrib.Value.Value
	}

	return m
}

func getEnvSubscriptionId(t *testing.T, dir string, envName string) string {
	envPath := filepath.Join(dir, azdcontext.EnvironmentDirectoryName, envName)
	env, err := environment.FromRoot(envPath)
	require.NoError(t, err)
	return env.GetSubscriptionId()
}

func verifyResource(t *testing.T, attributes []Attribute) {
	m := attributesMap(attributes)

	require.Contains(t, m, fields.MachineIdKey)
	machineId, ok := m[fields.MachineIdKey].(string)
	require.True(t, ok, "expected machine ID to be string type")
	isSha256 := Sha256Regex.MatchString(machineId)
	_, err := uuid.Parse(machineId)
	isUuid := err == nil
	require.True(t, isSha256 || isUuid, "invalid machine ID format. expected sha256 or uuid")

	require.Contains(t, m, fields.ServiceVersionKey)
	require.Equal(t, m[fields.ServiceVersionKey], getExpectedVersion(t))

	require.Contains(t, m, fields.ServiceVersionKey)
	require.Equal(t, m[fields.ServiceNameKey], fields.ServiceNameAzd)

	require.Contains(t, m, fields.ExecutionEnvironmentKey)

	if os.Getenv("BUILD_BUILDID") != "" {
		require.Equal(t, m[fields.ExecutionEnvironmentKey], fields.EnvAzurePipelines)
	} else if os.Getenv("GITHUB_RUN_ID") != "" {
		require.Equal(t, m[fields.ExecutionEnvironmentKey], fields.EnvGitHubActions)
	}

	require.Contains(t, m, fields.OSTypeKey)
	require.Contains(t, m, fields.OSVersionKey)
	require.Contains(t, m, fields.HostArchKey)
	require.Contains(t, m, fields.ProcessRuntimeVersionKey)
}

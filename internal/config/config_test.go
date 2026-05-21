package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// evalSymlinks is a helper that resolves symlinks for path comparison
// On macOS, /var is a symlink to /private/var which causes test failures
func evalSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If we can't resolve, just return the original
		return path
	}
	return resolved
}

func TestNestedRecordingSamplingConfig(t *testing.T) {
	defer Invalidate()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
recording:
  sampling:
    mode: adaptive
    base_rate: 0.25
    min_rate: 0.05
    log_transitions: false
`), 0o600))

	require.NoError(t, Load(configPath))

	cfg, err := Get()
	require.NoError(t, err)
	assert.Equal(t, "adaptive", cfg.Recording.Sampling.Mode)
	require.NotNil(t, cfg.Recording.Sampling.BaseRate)
	assert.Equal(t, 0.25, *cfg.Recording.Sampling.BaseRate)
	require.NotNil(t, cfg.Recording.Sampling.MinRate)
	assert.Equal(t, 0.05, *cfg.Recording.Sampling.MinRate)
	require.NotNil(t, cfg.Recording.Sampling.LogTransitions)
	assert.False(t, *cfg.Recording.Sampling.LogTransitions)
	assert.Equal(t, 0.25, cfg.Recording.SamplingRate)
}

func TestLegacyRecordingSamplingRateBackfillsNestedSamplingConfig(t *testing.T) {
	defer Invalidate()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
recording:
  sampling_rate: 0.25
`), 0o600))

	require.NoError(t, Load(configPath))

	cfg, err := Get()
	require.NoError(t, err)
	assert.Equal(t, 0.25, cfg.Recording.SamplingRate)
	assert.Equal(t, "fixed", cfg.Recording.Sampling.Mode)
	require.NotNil(t, cfg.Recording.Sampling.BaseRate)
	assert.Equal(t, 0.25, *cfg.Recording.Sampling.BaseRate)
	assert.Nil(t, cfg.Recording.Sampling.MinRate)
}

func TestRecordingSamplingRateEnvOverrideBeatsNestedBaseRate(t *testing.T) {
	defer Invalidate()
	t.Setenv("TUSK_RECORDING_SAMPLING_RATE", "0.5")
	t.Setenv("TUSK_RECORDING_SAMPLING_LOG_TRANSITIONS", "false")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
recording:
  sampling:
    mode: adaptive
    base_rate: 0.25
    min_rate: 0.05
    log_transitions: true
`), 0o600))

	require.NoError(t, Load(configPath))

	cfg, err := Get()
	require.NoError(t, err)
	require.NotNil(t, cfg.Recording.Sampling.BaseRate)
	assert.Equal(t, 0.5, *cfg.Recording.Sampling.BaseRate)
	assert.Equal(t, 0.5, cfg.Recording.SamplingRate)
	assert.Equal(t, "adaptive", cfg.Recording.Sampling.Mode)
	require.NotNil(t, cfg.Recording.Sampling.MinRate)
	assert.Equal(t, 0.05, *cfg.Recording.Sampling.MinRate)
	require.NotNil(t, cfg.Recording.Sampling.LogTransitions)
	assert.False(t, *cfg.Recording.Sampling.LogTransitions)
}

func TestLocalOverrideFileMergesOnTopOfBaseConfig(t *testing.T) {
	defer Invalidate()

	tmpDir := t.TempDir()
	tuskDir := filepath.Join(tmpDir, ".tusk")
	require.NoError(t, os.MkdirAll(tuskDir, 0o750))

	// Base config
	baseConfig := filepath.Join(tuskDir, "config.yaml")
	require.NoError(t, os.WriteFile(baseConfig, []byte(`
service:
  name: my-service
  port: 8080
  start:
    command: npm start
recording:
  sampling:
    mode: adaptive
    base_rate: 0.25
  export_spans: true
  enable_env_var_recording: true
`), 0o600))

	// Local override - only overrides recording settings
	localConfig := filepath.Join(tuskDir, "local-config.yaml")
	require.NoError(t, os.WriteFile(localConfig, []byte(`
recording:
  sampling:
    mode: fixed
    base_rate: 1.0
  export_spans: false
  enable_env_var_recording: false
`), 0o600))

	require.NoError(t, Load(baseConfig))

	cfg, err := Get()
	require.NoError(t, err)

	// Recording settings should be overridden by local config
	assert.Equal(t, "fixed", cfg.Recording.Sampling.Mode)
	require.NotNil(t, cfg.Recording.Sampling.BaseRate)
	assert.Equal(t, 1.0, *cfg.Recording.Sampling.BaseRate)
	require.NotNil(t, cfg.Recording.ExportSpans)
	assert.False(t, *cfg.Recording.ExportSpans)
	require.NotNil(t, cfg.Recording.EnableEnvVarRecording)
	assert.False(t, *cfg.Recording.EnableEnvVarRecording)

	// Service settings from base config should be preserved
	assert.Equal(t, "my-service", cfg.Service.Name)
	assert.Equal(t, 8080, cfg.Service.Port)
	assert.Equal(t, "npm start", cfg.Service.Start.Command)
}

func TestLocalOverrideFileYmlExtension(t *testing.T) {
	defer Invalidate()

	tmpDir := t.TempDir()
	tuskDir := filepath.Join(tmpDir, ".tusk")
	require.NoError(t, os.MkdirAll(tuskDir, 0o750))

	baseConfig := filepath.Join(tuskDir, "config.yaml")
	require.NoError(t, os.WriteFile(baseConfig, []byte(`
service:
  name: base-service
  port: 3000
  start:
    command: npm start
`), 0o600))

	// Use .yml extension for local override
	localConfig := filepath.Join(tuskDir, "local-config.yml")
	require.NoError(t, os.WriteFile(localConfig, []byte(`
service:
  name: local-service
`), 0o600))

	require.NoError(t, Load(baseConfig))

	cfg, err := Get()
	require.NoError(t, err)
	assert.Equal(t, "local-service", cfg.Service.Name)
	assert.Equal(t, 3000, cfg.Service.Port)
}

func TestTuskConfigOverrideEnvVar(t *testing.T) {
	defer Invalidate()

	tmpDir := t.TempDir()
	tuskDir := filepath.Join(tmpDir, ".tusk")
	require.NoError(t, os.MkdirAll(tuskDir, 0o750))

	baseConfig := filepath.Join(tuskDir, "config.yaml")
	require.NoError(t, os.WriteFile(baseConfig, []byte(`
service:
  name: base-service
  port: 3000
  start:
    command: npm start
recording:
  sampling:
    mode: adaptive
`), 0o600))

	// External override file (not in .tusk/ directory)
	overrideFile := filepath.Join(tmpDir, "my-override.yaml")
	require.NoError(t, os.WriteFile(overrideFile, []byte(`
recording:
  sampling:
    mode: fixed
    base_rate: 1.0
  export_spans: false
`), 0o600))

	t.Setenv("TUSK_CONFIG_OVERRIDE", overrideFile)

	require.NoError(t, Load(baseConfig))

	cfg, err := Get()
	require.NoError(t, err)

	// Override file takes precedence over base config
	assert.Equal(t, "fixed", cfg.Recording.Sampling.Mode)
	require.NotNil(t, cfg.Recording.Sampling.BaseRate)
	assert.Equal(t, 1.0, *cfg.Recording.Sampling.BaseRate)
	require.NotNil(t, cfg.Recording.ExportSpans)
	assert.False(t, *cfg.Recording.ExportSpans)

	// Base config values preserved for non-overridden fields
	assert.Equal(t, "base-service", cfg.Service.Name)
}

func TestTuskConfigOverrideEnvVarFileNotFound(t *testing.T) {
	defer Invalidate()

	t.Setenv("TUSK_CONFIG_OVERRIDE", "/nonexistent/override.yaml")

	err := Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TUSK_CONFIG_OVERRIDE file not found")
}

func TestTuskConfigOverrideBeatsLocalOverride(t *testing.T) {
	defer Invalidate()

	tmpDir := t.TempDir()
	tuskDir := filepath.Join(tmpDir, ".tusk")
	require.NoError(t, os.MkdirAll(tuskDir, 0o750))

	baseConfig := filepath.Join(tuskDir, "config.yaml")
	require.NoError(t, os.WriteFile(baseConfig, []byte(`
service:
  name: base
  port: 3000
  start:
    command: npm start
recording:
  sampling:
    mode: adaptive
`), 0o600))

	// Local override sets mode to fixed
	localConfig := filepath.Join(tuskDir, "local-config.yaml")
	require.NoError(t, os.WriteFile(localConfig, []byte(`
recording:
  sampling:
    mode: fixed
`), 0o600))

	// TUSK_CONFIG_OVERRIDE re-sets it back to adaptive with different rate
	overrideFile := filepath.Join(tmpDir, "env-override.yaml")
	require.NoError(t, os.WriteFile(overrideFile, []byte(`
recording:
  sampling:
    mode: adaptive
    base_rate: 0.5
`), 0o600))

	t.Setenv("TUSK_CONFIG_OVERRIDE", overrideFile)

	require.NoError(t, Load(baseConfig))

	cfg, err := Get()
	require.NoError(t, err)

	// TUSK_CONFIG_OVERRIDE wins over local-config.yaml
	assert.Equal(t, "adaptive", cfg.Recording.Sampling.Mode)
	require.NotNil(t, cfg.Recording.Sampling.BaseRate)
	assert.Equal(t, 0.5, *cfg.Recording.Sampling.BaseRate)
}

func TestRecordingSamplingModeEnvOverride(t *testing.T) {
	defer Invalidate()

	t.Setenv("TUSK_RECORDING_SAMPLING_MODE", "fixed")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
recording:
  sampling:
    mode: adaptive
    base_rate: 0.25
`), 0o600))

	require.NoError(t, Load(configPath))

	cfg, err := Get()
	require.NoError(t, err)
	assert.Equal(t, "fixed", cfg.Recording.Sampling.Mode)
}

func TestRecordingExportSpansEnvOverride(t *testing.T) {
	defer Invalidate()

	t.Setenv("TUSK_RECORDING_EXPORT_SPANS", "false")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
recording:
  export_spans: true
`), 0o600))

	require.NoError(t, Load(configPath))

	cfg, err := Get()
	require.NoError(t, err)
	require.NotNil(t, cfg.Recording.ExportSpans)
	assert.False(t, *cfg.Recording.ExportSpans)
}

func TestEnableEnvVarRecordingEnvOverride(t *testing.T) {
	defer Invalidate()

	t.Setenv("TUSK_ENABLE_ENV_VAR_RECORDING", "false")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
recording:
  enable_env_var_recording: true
`), 0o600))

	require.NoError(t, Load(configPath))

	cfg, err := Get()
	require.NoError(t, err)
	require.NotNil(t, cfg.Recording.EnableEnvVarRecording)
	assert.False(t, *cfg.Recording.EnableEnvVarRecording)
}

func TestEnvVarOverrideBeatsAllConfigFiles(t *testing.T) {
	defer Invalidate()

	tmpDir := t.TempDir()
	tuskDir := filepath.Join(tmpDir, ".tusk")
	require.NoError(t, os.MkdirAll(tuskDir, 0o750))

	baseConfig := filepath.Join(tuskDir, "config.yaml")
	require.NoError(t, os.WriteFile(baseConfig, []byte(`
service:
  name: test
  port: 3000
  start:
    command: npm start
recording:
  sampling:
    mode: adaptive
  export_spans: true
`), 0o600))

	localConfig := filepath.Join(tuskDir, "local-config.yaml")
	require.NoError(t, os.WriteFile(localConfig, []byte(`
recording:
  sampling:
    mode: fixed
`), 0o600))

	// Env vars win over everything
	t.Setenv("TUSK_RECORDING_SAMPLING_MODE", "adaptive")
	t.Setenv("TUSK_RECORDING_EXPORT_SPANS", "false")

	require.NoError(t, Load(baseConfig))

	cfg, err := Get()
	require.NoError(t, err)
	assert.Equal(t, "adaptive", cfg.Recording.Sampling.Mode)
	require.NotNil(t, cfg.Recording.ExportSpans)
	assert.False(t, *cfg.Recording.ExportSpans)
}

func TestFindLocalOverrideFile(t *testing.T) {
	tmpDir := t.TempDir()
	tuskDir := filepath.Join(tmpDir, ".tusk")
	require.NoError(t, os.MkdirAll(tuskDir, 0o750))

	baseConfig := filepath.Join(tuskDir, "config.yaml")
	require.NoError(t, os.WriteFile(baseConfig, []byte("service:\n  name: test"), 0o600))

	// No local override file exists
	assert.Equal(t, "", findLocalOverrideFile(baseConfig))

	// Create local-config.yaml
	localYaml := filepath.Join(tuskDir, "local-config.yaml")
	require.NoError(t, os.WriteFile(localYaml, []byte("service:\n  name: local"), 0o600))
	assert.Equal(t, localYaml, findLocalOverrideFile(baseConfig))

	// Remove .yaml and create .yml
	require.NoError(t, os.Remove(localYaml))
	localYml := filepath.Join(tuskDir, "local-config.yml")
	require.NoError(t, os.WriteFile(localYml, []byte("service:\n  name: local"), 0o600))
	assert.Equal(t, localYml, findLocalOverrideFile(baseConfig))
}

func TestValidateRejectsInvalidRecordingSamplingMode(t *testing.T) {
	cfg := &Config{
		Service: ServiceConfig{
			Port: 3000,
			Communication: CommunicationConfig{
				Type:    "auto",
				TCPPort: 9001,
			},
		},
		Recording: RecordingConfig{
			Sampling: RecordingSamplingConfig{
				Mode: "adapttive",
			},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "recording.sampling.mode must be 'fixed' or 'adaptive'")
}

func TestValidateRejectsOutOfRangeRecordingSamplingRates(t *testing.T) {
	baseRate := 5.0
	minRate := -0.1
	cfg := &Config{
		Service: ServiceConfig{
			Port: 3000,
			Communication: CommunicationConfig{
				Type:    "auto",
				TCPPort: 9001,
			},
		},
		Recording: RecordingConfig{
			Sampling: RecordingSamplingConfig{
				Mode:     "adaptive",
				BaseRate: &baseRate,
				MinRate:  &minRate,
			},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "recording.sampling.base_rate must be between 0.0 and 1.0")
	assert.ErrorContains(t, err, "recording.sampling.min_rate must be between 0.0 and 1.0")
}

func TestValidateRejectsRecordingMinRateGreaterThanBaseRate(t *testing.T) {
	baseRate := 0.1
	minRate := 0.9
	cfg := &Config{
		Service: ServiceConfig{
			Port: 3000,
			Communication: CommunicationConfig{
				Type:    "auto",
				TCPPort: 9001,
			},
		},
		Recording: RecordingConfig{
			Sampling: RecordingSamplingConfig{
				Mode:     "adaptive",
				BaseRate: &baseRate,
				MinRate:  &minRate,
			},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "recording.sampling.min_rate must be less than or equal to recording.sampling.base_rate")
}

func TestFindConfigFile_ParentTraversal(t *testing.T) {
	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	defer Invalidate()

	tmp := evalSymlinks(t.TempDir())
	configPath := filepath.Join(tmp, ".tusk", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o750))
	require.NoError(t, os.WriteFile(configPath, []byte("service:\n  name: test"), 0o600))

	// Work from a subdirectory
	subdir := filepath.Join(tmp, "src", "handlers")
	require.NoError(t, os.MkdirAll(subdir, 0o750))
	require.NoError(t, os.Chdir(subdir))

	found := findConfigFile()
	assert.Equal(t, configPath, found)
}

func TestFindConfigFile_ClosestWins(t *testing.T) {
	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	defer Invalidate()

	// Create structure: tmp/.tusk/config.yaml and tmp/nested/.tusk/config.yaml
	tmp := evalSymlinks(t.TempDir())
	outerConfig := filepath.Join(tmp, ".tusk", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(outerConfig), 0o750))
	require.NoError(t, os.WriteFile(outerConfig, []byte("service:\n  name: outer"), 0o600))

	nestedRoot := filepath.Join(tmp, "nested")
	innerConfig := filepath.Join(nestedRoot, ".tusk", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(innerConfig), 0o750))
	require.NoError(t, os.WriteFile(innerConfig, []byte("service:\n  name: inner"), 0o600))

	subdir := filepath.Join(nestedRoot, "src")
	require.NoError(t, os.MkdirAll(subdir, 0o750))
	require.NoError(t, os.Chdir(subdir))

	// Should find the closest config (in nested/.tusk/, not tmp/.tusk/)
	found := findConfigFile()
	assert.Equal(t, innerConfig, found)
}

func TestFindConfigFile_RootLevel(t *testing.T) {
	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	defer Invalidate()

	tmp := evalSymlinks(t.TempDir())
	configPath := filepath.Join(tmp, "tusk.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("service:\n  name: test"), 0o600))

	subdir := filepath.Join(tmp, "src")
	require.NoError(t, os.MkdirAll(subdir, 0o750))
	require.NoError(t, os.Chdir(subdir))

	found := findConfigFile()
	assert.Equal(t, configPath, found)
}

func TestConfigPaths_ResolvedRelativeToTuskRoot(t *testing.T) {
	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	defer Invalidate()

	tmp := evalSymlinks(t.TempDir())
	tuskDir := filepath.Join(tmp, ".tusk")
	require.NoError(t, os.MkdirAll(tuskDir, 0o750))

	// Write config with relative paths
	configPath := filepath.Join(tuskDir, "config.yaml")
	configContent := `service:
  name: test-service
  port: 8080
results:
  dir: .tusk/results
traces:
  dir: .tusk/traces
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	// Change to a nested directory
	subdir := filepath.Join(tmp, "src", "api")
	require.NoError(t, os.MkdirAll(subdir, 0o750))
	require.NoError(t, os.Chdir(subdir))

	// Load config
	err := Load("")
	require.NoError(t, err)

	cfg, err := Get()
	require.NoError(t, err)

	// Paths should be resolved relative to tusk root (tmp), not current directory (tmp/src/api)
	assert.Equal(t, filepath.Join(tmp, ".tusk/results"), cfg.Results.Dir)
	assert.Equal(t, filepath.Join(tmp, ".tusk/traces"), cfg.Traces.Dir)
}

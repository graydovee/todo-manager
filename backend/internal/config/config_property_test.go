package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: ai-summary, Property 2: LLM config round-trip with environment variable precedence
// **Validates: Requirements 7.1, 7.2**
//
// Property: For any valid LLM configuration values (model, base_url, api_key, timeout)
// written to a YAML config, loading the config SHALL preserve all field values.
// Furthermore, for any environment variable override (TODOLIST_LLM_MODEL,
// TODOLIST_LLM_BASE_URL, TODOLIST_LLM_API_KEY, TODOLIST_LLM_TIMEOUT), the environment
// variable value SHALL take precedence over the YAML value in the loaded config.
func TestProperty_LLMConfigRoundTripWithEnvPrecedence(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random LLM config values
		model := rapid.StringMatching(`[a-z][a-z0-9\-]{1,20}`).Draw(rt, "model")
		baseURL := rapid.StringMatching(`https?://[a-z][a-z0-9]{1,10}\.[a-z]{2,4}`).Draw(rt, "baseURL")
		apiKey := rapid.StringMatching(`sk-[a-zA-Z0-9]{10,30}`).Draw(rt, "apiKey")
		timeout := rapid.IntRange(1, 300).Draw(rt, "timeout")

		// Write a minimal valid YAML config file
		yamlContent := fmt.Sprintf(`server:
  port: 8080
db:
  driver: sqlite
  dsn: test.db
auth:
  mode: basic
  basic:
    users:
      - username: test
        password: test
        display_name: Test
session:
  secret: testsecret123456
llm:
  model: %s
  base_url: %s
  api_key: %s
  timeout: %d
`, model, baseURL, apiKey, timeout)

		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
			rt.Fatalf("write config: %v", err)
		}

		// Clear any LLM env vars before testing YAML round-trip
		os.Unsetenv("TODOLIST_LLM_MODEL")
		os.Unsetenv("TODOLIST_LLM_BASE_URL")
		os.Unsetenv("TODOLIST_LLM_API_KEY")
		os.Unsetenv("TODOLIST_LLM_TIMEOUT")

		// Load config and verify YAML values are preserved
		cfg, err := Load(cfgPath)
		if err != nil {
			rt.Fatalf("load config: %v", err)
		}

		if cfg.LLM.Model != model {
			rt.Fatalf("model mismatch: expected %q, got %q", model, cfg.LLM.Model)
		}
		if cfg.LLM.BaseURL != baseURL {
			rt.Fatalf("base_url mismatch: expected %q, got %q", baseURL, cfg.LLM.BaseURL)
		}
		if cfg.LLM.APIKey != apiKey {
			rt.Fatalf("api_key mismatch: expected %q, got %q", apiKey, cfg.LLM.APIKey)
		}
		if cfg.LLM.Timeout != timeout {
			rt.Fatalf("timeout mismatch: expected %d, got %d", timeout, cfg.LLM.Timeout)
		}

		// Now test environment variable precedence
		// Generate different values for env overrides
		envModel := rapid.StringMatching(`[a-z][a-z0-9\-]{1,20}`).Draw(rt, "envModel")
		envBaseURL := rapid.StringMatching(`https?://[a-z][a-z0-9]{1,10}\.[a-z]{2,4}`).Draw(rt, "envBaseURL")
		envAPIKey := rapid.StringMatching(`sk-[a-zA-Z0-9]{10,30}`).Draw(rt, "envAPIKey")
		envTimeout := rapid.IntRange(1, 300).Draw(rt, "envTimeout")

		// Decide which env vars to set (at least one)
		setModel := rapid.Bool().Draw(rt, "setEnvModel")
		setBaseURL := rapid.Bool().Draw(rt, "setEnvBaseURL")
		setAPIKey := rapid.Bool().Draw(rt, "setEnvAPIKey")
		setTimeout := rapid.Bool().Draw(rt, "setEnvTimeout")

		// Ensure at least one env var is set
		if !setModel && !setBaseURL && !setAPIKey && !setTimeout {
			setModel = true
		}

		if setModel {
			os.Setenv("TODOLIST_LLM_MODEL", envModel)
			defer os.Unsetenv("TODOLIST_LLM_MODEL")
		}
		if setBaseURL {
			os.Setenv("TODOLIST_LLM_BASE_URL", envBaseURL)
			defer os.Unsetenv("TODOLIST_LLM_BASE_URL")
		}
		if setAPIKey {
			os.Setenv("TODOLIST_LLM_API_KEY", envAPIKey)
			defer os.Unsetenv("TODOLIST_LLM_API_KEY")
		}
		if setTimeout {
			os.Setenv("TODOLIST_LLM_TIMEOUT", fmt.Sprintf("%d", envTimeout))
			defer os.Unsetenv("TODOLIST_LLM_TIMEOUT")
		}

		// Reload config with env vars set
		cfg2, err := Load(cfgPath)
		if err != nil {
			rt.Fatalf("load config with env: %v", err)
		}

		// Verify env vars take precedence where set, YAML values preserved where not
		expectedModel := model
		if setModel {
			expectedModel = envModel
		}
		if cfg2.LLM.Model != expectedModel {
			rt.Fatalf("model with env: expected %q, got %q (setEnv=%v)", expectedModel, cfg2.LLM.Model, setModel)
		}

		expectedBaseURL := baseURL
		if setBaseURL {
			expectedBaseURL = envBaseURL
		}
		if cfg2.LLM.BaseURL != expectedBaseURL {
			rt.Fatalf("base_url with env: expected %q, got %q (setEnv=%v)", expectedBaseURL, cfg2.LLM.BaseURL, setBaseURL)
		}

		expectedAPIKey := apiKey
		if setAPIKey {
			expectedAPIKey = envAPIKey
		}
		if cfg2.LLM.APIKey != expectedAPIKey {
			rt.Fatalf("api_key with env: expected %q, got %q (setEnv=%v)", expectedAPIKey, cfg2.LLM.APIKey, setAPIKey)
		}

		expectedTimeout := timeout
		if setTimeout {
			expectedTimeout = envTimeout
		}
		if cfg2.LLM.Timeout != expectedTimeout {
			rt.Fatalf("timeout with env: expected %d, got %d (setEnv=%v)", expectedTimeout, cfg2.LLM.Timeout, setTimeout)
		}
	})
}

// Feature: ai-summary, Property 3: Missing LLM config fields produce specific error messages
// **Validates: Requirements 7.3**
//
// Property: For any combination of missing or empty required LLM config fields
// (model, base_url, api_key), when an AI summary is requested, the service SHALL
// return an error message that identifies which specific field(s) are missing.
func TestProperty_MissingLLMConfigFieldsProduceSpecificErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random valid values for each field
		validModel := rapid.StringMatching(`[a-z][a-z0-9\-]{1,20}`).Draw(rt, "validModel")
		validBaseURL := rapid.StringMatching(`https?://[a-z][a-z0-9]{1,10}\.[a-z]{2,4}`).Draw(rt, "validBaseURL")
		validAPIKey := rapid.StringMatching(`sk-[a-zA-Z0-9]{10,30}`).Draw(rt, "validAPIKey")

		// Decide which fields to make missing/empty (at least one must be missing)
		missingModel := rapid.Bool().Draw(rt, "missingModel")
		missingBaseURL := rapid.Bool().Draw(rt, "missingBaseURL")
		missingAPIKey := rapid.Bool().Draw(rt, "missingAPIKey")

		// Ensure at least one field is missing
		if !missingModel && !missingBaseURL && !missingAPIKey {
			// Force at least one to be missing
			choice := rapid.IntRange(0, 2).Draw(rt, "forceMissing")
			switch choice {
			case 0:
				missingModel = true
			case 1:
				missingBaseURL = true
			case 2:
				missingAPIKey = true
			}
		}

		// Build the LLM config with some fields missing/empty
		cfg := &LLMConfig{
			Timeout: 30,
		}

		if !missingModel {
			cfg.Model = validModel
		} else {
			// Randomly choose between empty string and whitespace-only
			if rapid.Bool().Draw(rt, "modelWhitespace") {
				cfg.Model = "   "
			} else {
				cfg.Model = ""
			}
		}

		if !missingBaseURL {
			cfg.BaseURL = validBaseURL
		} else {
			if rapid.Bool().Draw(rt, "baseURLWhitespace") {
				cfg.BaseURL = "  "
			} else {
				cfg.BaseURL = ""
			}
		}

		if !missingAPIKey {
			cfg.APIKey = validAPIKey
		} else {
			if rapid.Bool().Draw(rt, "apiKeyWhitespace") {
				cfg.APIKey = "  "
			} else {
				cfg.APIKey = ""
			}
		}

		// Call ValidateLLMConfig
		err := ValidateLLMConfig(cfg)

		// Must return an error since at least one field is missing
		if err == nil {
			rt.Fatalf("expected error for missing fields (model=%v, base_url=%v, api_key=%v), got nil",
				missingModel, missingBaseURL, missingAPIKey)
		}

		errMsg := err.Error()

		// Verify the error message identifies each missing field specifically
		if missingModel {
			if !strings.Contains(errMsg, "model") {
				rt.Fatalf("error should mention 'model' when model is missing: %q", errMsg)
			}
		} else {
			if strings.Contains(errMsg, "model") {
				rt.Fatalf("error should NOT mention 'model' when model is present: %q", errMsg)
			}
		}

		if missingBaseURL {
			if !strings.Contains(errMsg, "base_url") {
				rt.Fatalf("error should mention 'base_url' when base_url is missing: %q", errMsg)
			}
		} else {
			if strings.Contains(errMsg, "base_url") {
				rt.Fatalf("error should NOT mention 'base_url' when base_url is present: %q", errMsg)
			}
		}

		if missingAPIKey {
			if !strings.Contains(errMsg, "api_key") {
				rt.Fatalf("error should mention 'api_key' when api_key is missing: %q", errMsg)
			}
		} else {
			if strings.Contains(errMsg, "api_key") {
				rt.Fatalf("error should NOT mention 'api_key' when api_key is present: %q", errMsg)
			}
		}

		// Also verify that when ALL fields are present, no error is returned
		fullCfg := &LLMConfig{
			Model:   validModel,
			BaseURL: validBaseURL,
			APIKey:  validAPIKey,
			Timeout: 30,
		}
		if err := ValidateLLMConfig(fullCfg); err != nil {
			rt.Fatalf("expected no error with all fields present, got: %v", err)
		}
	})
}

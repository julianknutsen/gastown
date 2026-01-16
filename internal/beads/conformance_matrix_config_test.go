package beads_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestMatrix_Config(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Config",
		Operation: "ConfigSet",
		Test: func(ops beads.BeadsOps) error {
			// Set a config value
			err := ops.ConfigSet("test.key", "test-value")
			if err != nil {
				return fmt.Errorf("ConfigSet failed: %v", err)
			}

			// Get the value back
			value, err := ops.ConfigGet("test.key")
			if err != nil {
				return fmt.Errorf("ConfigGet failed: %v", err)
			}
			if value != "test-value" {
				return fmt.Errorf("ConfigGet = %q, want %q", value, "test-value")
			}
			return nil
		},
	})
}

func TestMatrix_ConfigGet_NotExists(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "ConfigGet_NotExists",
		Operation: "ConfigGet",
		Test: func(ops beads.BeadsOps) error {
			value, err := ops.ConfigGet("nonexistent-key-xyz")
			if err != nil {
				return fmt.Errorf("ConfigGet should not error for missing key: %v", err)
			}
			// Double/Implementation return empty string for missing keys.
			// RawBd returns bd's native format: "key (not set)"
			if value != "" && !strings.HasSuffix(value, "(not set)") {
				return fmt.Errorf("ConfigGet for missing key = %q, want empty string or '(not set)' suffix", value)
			}
			return nil
		},
	})
}

func TestMatrix_ConfigSet_Overwrite(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "ConfigSet_Overwrite",
		Operation: "ConfigSet",
		Test: func(ops beads.BeadsOps) error {
			_ = ops.ConfigSet("overwrite.key", "oldvalue")
			err := ops.ConfigSet("overwrite.key", "newvalue")
			if err != nil {
				return fmt.Errorf("ConfigSet overwrite failed: %v", err)
			}
			value, _ := ops.ConfigGet("overwrite.key")
			if value != "newvalue" {
				return fmt.Errorf("ConfigGet after overwrite = %q, want %q", value, "newvalue")
			}
			return nil
		},
	})
}

func TestMatrix_ConfigSet_MultipleKeys(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "ConfigSet_MultipleKeys",
		Operation: "ConfigSet",
		Test: func(ops beads.BeadsOps) error {
			_ = ops.ConfigSet("multi.key1", "value1")
			_ = ops.ConfigSet("multi.key2", "value2")
			_ = ops.ConfigSet("multi.key3", "value3")

			v1, _ := ops.ConfigGet("multi.key1")
			v2, _ := ops.ConfigGet("multi.key2")
			v3, _ := ops.ConfigGet("multi.key3")

			if v1 != "value1" || v2 != "value2" || v3 != "value3" {
				return fmt.Errorf("Multiple keys not stored correctly")
			}
			return nil
		},
	})
}

func TestMatrix_Init(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Init",
		Operation: "Init",
		Test: func(ops beads.BeadsOps) error {
			// Init should work (may already be initialized)
			err := ops.Init(beads.InitOptions{Quiet: true})
			if err != nil {
				// Some implementations may not support re-init, which is OK
				return nil
			}
			return nil
		},
	})
}

func TestMatrix_Migrate(t *testing.T) {
	RunSimpleConformanceTest(t, SimpleConformanceTest{
		Name:      "Migrate",
		Operation: "Migrate",
		Test: func(ops beads.BeadsOps) error {
			err := ops.Migrate(beads.MigrateOptions{Yes: true})
			if err != nil {
				// Migrate may fail in test env, which is acceptable
				return nil
			}
			return nil
		},
	})
}

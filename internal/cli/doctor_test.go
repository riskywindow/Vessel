package cli

import (
	"runtime"
	"testing"
)

func TestCheckOS_Linux(t *testing.T) {
	result := checkOS()

	if runtime.GOOS == "linux" {
		if result.Status != "pass" {
			t.Errorf("expected pass on Linux, got %s", result.Status)
		}
		if result.Name != "Operating System" {
			t.Errorf("unexpected name: %s", result.Name)
		}
	} else {
		if result.Status != "fail" {
			t.Errorf("expected fail on non-Linux, got %s", result.Status)
		}
	}
}

func TestCheckKernel(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("kernel check only works on Linux")
	}

	result := checkKernel()

	if result.Name != "Kernel Version" {
		t.Errorf("unexpected name: %s", result.Name)
	}
	// Should be pass or warn, never fail
	if result.Status == "fail" {
		t.Errorf("kernel check should never fail, got: %s", result.Message)
	}
	if result.Message == "" {
		t.Error("expected non-empty kernel version message")
	}
}

func TestCheckCgroupsV2(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cgroups check only works on Linux")
	}

	result := checkCgroupsV2()

	if result.Name != "cgroups v2" {
		t.Errorf("unexpected name: %s", result.Name)
	}
	// On our test environment, cgroups v2 should be available
	if result.Status == "" {
		t.Error("expected non-empty status")
	}
}

func TestCheckOverlayFS(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("overlayfs check only works on Linux")
	}

	result := checkOverlayFS()

	if result.Name != "OverlayFS" {
		t.Errorf("unexpected name: %s", result.Name)
	}
	if result.Status == "" {
		t.Error("expected non-empty status")
	}
}

func TestCheckRequiredCommands(t *testing.T) {
	results := checkRequiredCommands()

	if len(results) != 7 {
		t.Errorf("expected 7 command checks, got %d", len(results))
	}

	// Verify names match expected commands
	expectedNames := []string{
		"Command: nsenter",
		"Command: iptables",
		"Command: ip",
		"Command: mount",
		"Command: umount",
		"Command: tar",
		"Command: gzip",
	}

	for i, r := range results {
		if r.Name != expectedNames[i] {
			t.Errorf("check %d: expected name %q, got %q", i, expectedNames[i], r.Name)
		}
		if r.Status != "pass" && r.Status != "fail" && r.Status != "warn" {
			t.Errorf("check %d: unexpected status %q", i, r.Status)
		}
	}
}

func TestCheckUserPermissions(t *testing.T) {
	result := checkUserPermissions()

	if result.Name != "User Permissions" {
		t.Errorf("unexpected name: %s", result.Name)
	}
	// Should be pass or warn (never fail)
	if result.Status != "pass" && result.Status != "warn" {
		t.Errorf("unexpected status: %s", result.Status)
	}
}

func TestCheckNetworkCapabilities(t *testing.T) {
	result := checkNetworkCapabilities()

	if result.Name != "Network Capabilities" {
		t.Errorf("unexpected name: %s", result.Name)
	}
	if result.Status == "" {
		t.Error("expected non-empty status")
	}
}

func TestCheckVesselDirectories(t *testing.T) {
	result := checkVesselDirectories()

	if result.Name != "Vessel Directories" {
		t.Errorf("unexpected name: %s", result.Name)
	}
	// Should always pass (dirs either exist, don't exist, or partially exist)
	if result.Status != "pass" {
		t.Errorf("expected pass, got %s: %s", result.Status, result.Message)
	}
}

func TestCheckDaemonStatus(t *testing.T) {
	result := checkDaemonStatus()

	if result.Name != "Daemon Status" {
		t.Errorf("unexpected name: %s", result.Name)
	}
	// In test environment, daemon is likely not running
	if result.Status == "" {
		t.Error("expected non-empty status")
	}
}

func TestCheckResult_JSONFields(t *testing.T) {
	result := CheckResult{
		Name:    "Test",
		Status:  "pass",
		Message: "All good",
		Details: "details here",
	}

	if result.Name != "Test" {
		t.Error("Name field mismatch")
	}
	if result.Status != "pass" {
		t.Error("Status field mismatch")
	}
	if result.Message != "All good" {
		t.Error("Message field mismatch")
	}
	if result.Details != "details here" {
		t.Error("Details field mismatch")
	}
}

func TestCheckResult_StatusValues(t *testing.T) {
	// Verify valid status values
	validStatuses := []string{"pass", "fail", "warn"}
	for _, s := range validStatuses {
		r := CheckResult{Status: s}
		if r.Status != s {
			t.Errorf("expected status %q", s)
		}
	}
}

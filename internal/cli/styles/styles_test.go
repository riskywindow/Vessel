package styles

import (
	"os"
	"testing"
)

func TestColorEnabled_Default(t *testing.T) {
	// Save and restore NO_COLOR
	old := os.Getenv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if old != "" {
			os.Setenv("NO_COLOR", old)
		}
	}()

	if !ColorEnabled() {
		t.Error("expected ColorEnabled() to return true when NO_COLOR is not set")
	}
}

func TestColorEnabled_NoColor(t *testing.T) {
	old := os.Getenv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if old == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", old)
		}
	}()

	if ColorEnabled() {
		t.Error("expected ColorEnabled() to return false when NO_COLOR is set")
	}
}

func TestRender_WithColor(t *testing.T) {
	old := os.Getenv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if old != "" {
			os.Setenv("NO_COLOR", old)
		}
	}()

	result := Render(Success, "hello")
	if result == "" {
		t.Error("expected non-empty render result")
	}
}

func TestRender_NoColor(t *testing.T) {
	old := os.Getenv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if old == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", old)
		}
	}()

	result := Render(Success, "hello")
	if result != "hello" {
		t.Errorf("expected plain 'hello' when NO_COLOR set, got %q", result)
	}
}

func TestSuccessIcon(t *testing.T) {
	icon := SuccessIcon()
	if icon == "" {
		t.Error("expected non-empty success icon")
	}
}

func TestErrorIcon(t *testing.T) {
	icon := ErrorIcon()
	if icon == "" {
		t.Error("expected non-empty error icon")
	}
}

func TestWarningIcon(t *testing.T) {
	icon := WarningIcon()
	if icon == "" {
		t.Error("expected non-empty warning icon")
	}
}

func TestInfoIcon(t *testing.T) {
	icon := InfoIcon()
	if icon == "" {
		t.Error("expected non-empty info icon")
	}
}

func TestCheckPassIcon(t *testing.T) {
	icon := CheckPassIcon()
	if icon == "" {
		t.Error("expected non-empty check pass icon")
	}
}

func TestCheckFailIcon(t *testing.T) {
	icon := CheckFailIcon()
	if icon == "" {
		t.Error("expected non-empty check fail icon")
	}
}

func TestCheckWarnIcon(t *testing.T) {
	icon := CheckWarnIcon()
	if icon == "" {
		t.Error("expected non-empty check warn icon")
	}
}

func TestStatusDot_Running(t *testing.T) {
	dot := StatusDot("running")
	if dot == "" {
		t.Error("expected non-empty status dot for running")
	}
}

func TestStatusDot_Stopped(t *testing.T) {
	dot := StatusDot("stopped")
	if dot == "" {
		t.Error("expected non-empty status dot for stopped")
	}
}

func TestStatusDot_Failed(t *testing.T) {
	dot := StatusDot("failed")
	if dot == "" {
		t.Error("expected non-empty status dot for failed")
	}
}

func TestStatusDot_Healthy(t *testing.T) {
	dot := StatusDot("healthy")
	if dot == "" {
		t.Error("expected non-empty status dot for healthy")
	}
}

func TestStatusDot_Unknown(t *testing.T) {
	dot := StatusDot("something")
	if dot == "" {
		t.Error("expected non-empty status dot for unknown")
	}
}

func TestStylesExist(t *testing.T) {
	// Verify all exported styles can be used without panic
	tests := []struct {
		name  string
		style func() string
	}{
		{"Success", func() string { return Success.Render("test") }},
		{"Error", func() string { return Error.Render("test") }},
		{"Warning", func() string { return Warning.Render("test") }},
		{"Info", func() string { return Info.Render("test") }},
		{"Muted", func() string { return Muted.Render("test") }},
		{"Bold", func() string { return Bold.Render("test") }},
		{"AppName", func() string { return AppName.Render("test") }},
		{"ContainerID", func() string { return ContainerID.Render("test") }},
		{"Version", func() string { return Version.Render("test") }},
		{"Header", func() string { return Header.Render("test") }},
		{"TableHeader", func() string { return TableHeader.Render("test") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.style()
			if result == "" {
				t.Errorf("style %s produced empty output", tt.name)
			}
		})
	}
}

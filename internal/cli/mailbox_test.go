package cli

import (
	"testing"
)

func TestMailboxListCmdRunWithoutEmail(t *testing.T) {
	cmd := &MailboxListCmd{}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	// Should return error when email not configured
	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailboxCreateCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailboxCreateCmd{
		Name: "TestFolder",
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailboxDeleteCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailboxDeleteCmd{
		Name: "TestFolder",
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestFormatAttributesEmpty(t *testing.T) {
	result := formatAttributes([]string{})
	if result != "" {
		t.Errorf("formatAttributes([]) = %q, want empty string", result)
	}
}

func TestFormatAttributesSingle(t *testing.T) {
	result := formatAttributes([]string{"\\HasChildren"})
	if result != "HasChildren" {
		t.Errorf("formatAttributes() = %q, want %q", result, "HasChildren")
	}
}

func TestFormatAttributesMultiple(t *testing.T) {
	result := formatAttributes([]string{"\\HasChildren", "\\Noselect", "\\Drafts"})
	expected := "HasChildren, Noselect, Drafts"
	if result != expected {
		t.Errorf("formatAttributes() = %q, want %q", result, expected)
	}
}

func TestFormatAttributesNoBackslash(t *testing.T) {
	result := formatAttributes([]string{"Custom", "Another"})
	expected := "Custom, Another"
	if result != expected {
		t.Errorf("formatAttributes() = %q, want %q", result, expected)
	}
}

func TestFormatAttributesMixed(t *testing.T) {
	result := formatAttributes([]string{"\\System", "Custom"})
	expected := "System, Custom"
	if result != expected {
		t.Errorf("formatAttributes() = %q, want %q", result, expected)
	}
}

func TestMailboxListCmdStruct(t *testing.T) {
	cmd := MailboxListCmd{}
	// Should be an empty struct (no fields)
	_ = cmd
}

func TestMailboxCreateCmdStruct(t *testing.T) {
	cmd := MailboxCreateCmd{
		Name: "Folder/Subfolder",
	}

	if cmd.Name != "Folder/Subfolder" {
		t.Errorf("Name = %q, want %q", cmd.Name, "Folder/Subfolder")
	}
}

func TestMailboxDeleteCmdStruct(t *testing.T) {
	cmd := MailboxDeleteCmd{
		Name: "OldFolder",
	}

	if cmd.Name != "OldFolder" {
		t.Errorf("Name = %q, want %q", cmd.Name, "OldFolder")
	}
}

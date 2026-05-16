package collab_test

import (
	"testing"

	"github.com/zero-agent/core/internal/collab"
)

func TestCanApproveToolPermission(t *testing.T) {
	room := &collab.Room{RequireHostApprovalDangerTools: true}

	if !collab.CanApproveToolPermission(collab.RoleHost, room, "bash") {
		t.Fatal("host should approve dangerous tools")
	}
	if !collab.CanApproveToolPermission(collab.RoleMaintainer, room, "read") {
		t.Fatal("maintainer should approve safe tools")
	}
	if collab.CanApproveToolPermission(collab.RoleMaintainer, room, "bash") {
		t.Fatal("maintainer should not approve dangerous tools when host approval required")
	}
	if collab.CanApproveToolPermission(collab.RolePrompter, room, "read") {
		t.Fatal("prompter should not approve tools")
	}
	if collab.CanApproveToolPermission(collab.RoleViewer, room, "read") {
		t.Fatal("viewer should not approve tools")
	}
}

func TestMaintainerCanApproveDangerousToolsWhenConfigured(t *testing.T) {
	room := &collab.Room{RequireHostApprovalDangerTools: false}

	if !collab.CanApproveToolPermission(collab.RoleMaintainer, room, "bash") {
		t.Fatal("maintainer should approve dangerous tools when host approval is disabled")
	}
}

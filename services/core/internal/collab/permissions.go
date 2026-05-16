package collab

type ToolRisk string

const (
	ToolRiskSafe      ToolRisk = "safe"
	ToolRiskDangerous ToolRisk = "dangerous"
)

func RiskForTool(toolName string) ToolRisk {
	switch toolName {
	case "bash", "write", "edit", "fetch":
		return ToolRiskDangerous
	default:
		return ToolRiskSafe
	}
}

func CanApproveToolPermission(role Role, room *Room, toolName string) bool {
	risk := RiskForTool(toolName)
	if role == RoleHost {
		return true
	}
	if role == RoleMaintainer && risk == ToolRiskSafe {
		return HasCapability(role, CapApproveSafe)
	}
	if role == RoleMaintainer && risk == ToolRiskDangerous && !room.RequireHostApprovalDangerTools {
		return HasCapability(role, CapApproveSafe)
	}
	return false
}

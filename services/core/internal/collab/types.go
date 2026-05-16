package collab

type Role string

const (
	RoleHost       Role = "host"
	RoleMaintainer Role = "maintainer"
	RolePrompter   Role = "prompter"
	RoleViewer     Role = "viewer"
)

type Capability string

const (
	CapSubmitPrompt    Capability = "prompt.submit"
	CapInterceptPrompt Capability = "prompt.intercept"
	CapApprovePrompt   Capability = "prompt.approve"
	CapEditPrompt      Capability = "prompt.edit"
	CapRejectPrompt    Capability = "prompt.reject"
	CapCancelPrompt    Capability = "prompt.cancel"
	CapApproveDanger   Capability = "tool.approve_dangerous"
	CapApproveSafe     Capability = "tool.approve_safe"
	CapManageRoles     Capability = "room.manage_roles"
	CapRevokeRoom      Capability = "room.revoke"
)

var RoleCapabilities = map[Role][]Capability{
	RoleHost: {
		CapSubmitPrompt, CapInterceptPrompt, CapApprovePrompt,
		CapEditPrompt, CapRejectPrompt, CapCancelPrompt,
		CapApproveDanger, CapApproveSafe, CapManageRoles, CapRevokeRoom,
	},
	RoleMaintainer: {
		CapSubmitPrompt, CapInterceptPrompt, CapApprovePrompt,
		CapEditPrompt, CapRejectPrompt, CapApproveSafe,
	},
	RolePrompter: {
		CapSubmitPrompt,
	},
	RoleViewer: {},
}

func HasCapability(role Role, cap Capability) bool {
	caps, ok := RoleCapabilities[role]
	if !ok {
		return false
	}
	for _, c := range caps {
		if c == cap {
			return true
		}
	}
	return false
}

type RoomStatus string

const (
	RoomActive   RoomStatus = "active"
	RoomRevoked  RoomStatus = "revoked"
	RoomArchived RoomStatus = "archived"
)

type ParticipantStatus string

const (
	ParticipantOnline  ParticipantStatus = "online"
	ParticipantOffline ParticipantStatus = "offline"
	ParticipantRemoved ParticipantStatus = "removed"
)

type PromptReviewMode string

const (
	ReviewOff             PromptReviewMode = "off"
	ReviewHostOnly        PromptReviewMode = "host_only"
	ReviewMaintainerOrHost PromptReviewMode = "maintainer_or_host"
	ReviewAll             PromptReviewMode = "all"
)

type PromptStatus string

const (
	PromptSubmitted     PromptStatus = "submitted"
	PromptPendingReview PromptStatus = "pending_review"
	PromptApproved      PromptStatus = "approved"
	PromptRejected      PromptStatus = "rejected"
	PromptQueued        PromptStatus = "queued"
	PromptRunning       PromptStatus = "running"
	PromptCompleted     PromptStatus = "completed"
	PromptCancelled     PromptStatus = "cancelled"
	PromptFailed        PromptStatus = "failed"
)

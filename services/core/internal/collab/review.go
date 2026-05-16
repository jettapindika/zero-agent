package collab

func RequiresReview(mode PromptReviewMode, actorRole Role) bool {
	switch mode {
	case ReviewOff:
		return false
	case ReviewHostOnly:
		return actorRole != RoleHost
	case ReviewMaintainerOrHost:
		return actorRole == RolePrompter || actorRole == RoleViewer
	case ReviewAll:
		return actorRole != RoleHost
	default:
		return false
	}
}

func CanReview(actorRole Role, room *Room) bool {
	if actorRole == RoleHost {
		return true
	}
	if actorRole == RoleMaintainer && room.AllowMaintainerPromptIntercept {
		return HasCapability(actorRole, CapInterceptPrompt)
	}
	return false
}

func EffectiveContent(item *PromptQueueItem) string {
	if item.ReviewedContent != nil && *item.ReviewedContent != "" {
		return *item.ReviewedContent
	}
	return item.Content
}

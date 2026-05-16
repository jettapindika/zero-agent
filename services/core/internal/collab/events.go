package collab

const (
	EventRoomCreated  = "collab.room.created"
	EventRoomUpdated  = "collab.room.updated"
	EventRoomRevoked  = "collab.room.revoked"

	EventParticipantJoined  = "collab.participant.joined"
	EventParticipantUpdated = "collab.participant.updated"
	EventParticipantLeft    = "collab.participant.left"

	EventPresenceUpdated = "collab.presence.updated"

	EventPromptSubmitted       = "prompt.submitted"
	EventPromptReviewRequired  = "prompt.review.required"
	EventPromptReviewApproved  = "prompt.review.approved"
	EventPromptReviewRejected  = "prompt.review.rejected"
	EventPromptReviewEdited    = "prompt.review.edited"
	EventPromptReviewCancelled = "prompt.review.cancelled"
	EventPromptQueued          = "prompt.queued"
	EventPromptStarted         = "prompt.started"
	EventPromptCompleted       = "prompt.completed"
	EventPromptCancelled       = "prompt.cancelled"
	EventPromptFailed          = "prompt.failed"

	EventSessionLocked   = "session.locked"
	EventSessionUnlocked = "session.unlocked"
)

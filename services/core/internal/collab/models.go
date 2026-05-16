package collab

type Room struct {
	ID                              string           `json:"id"`
	ProjectID                       string           `json:"projectId"`
	HostClientID                    string           `json:"hostClientId"`
	Name                            string           `json:"name,omitempty"`
	InviteTokenHash                 string           `json:"-"`
	Status                          RoomStatus       `json:"status"`
	DefaultRole                     Role             `json:"defaultRole"`
	PromptReviewMode                PromptReviewMode `json:"promptReviewMode"`
	AllowMaintainerPromptIntercept  bool             `json:"allowMaintainerPromptIntercept"`
	AllowPromptEditBeforeApproval   bool             `json:"allowPromptEditBeforeApproval"`
	RequireHostApprovalDangerTools  bool             `json:"requireHostApprovalDangerousTools"`
	AutoRunQueue                    bool             `json:"autoRunQueue"`
	CreatedAt                       int64            `json:"createdAt"`
	UpdatedAt                       int64            `json:"updatedAt"`
	RevokedAt                       *int64           `json:"revokedAt,omitempty"`
}

type Participant struct {
	ID          string            `json:"id"`
	RoomID      string            `json:"roomId"`
	ClientID    string            `json:"clientId"`
	DisplayName string            `json:"displayName"`
	Role        Role              `json:"role"`
	Status      ParticipantStatus `json:"status"`
	JoinedAt    int64             `json:"joinedAt"`
	LastSeenAt  int64             `json:"lastSeenAt"`
}

type PromptQueueItem struct {
	ID                string       `json:"id"`
	RoomID            *string      `json:"roomId,omitempty"`
	SessionID         string       `json:"sessionId"`
	ActorClientID     string       `json:"actorClientId"`
	Content           string       `json:"content"`
	OriginalContent   *string      `json:"originalContent,omitempty"`
	ReviewedContent   *string      `json:"reviewedContent,omitempty"`
	Status            PromptStatus `json:"status"`
	RequiresReview    bool         `json:"requiresReview"`
	ReviewedByClientID *string     `json:"reviewedByClientId,omitempty"`
	ReviewedAt        *int64       `json:"reviewedAt,omitempty"`
	ReviewNote        *string      `json:"reviewNote,omitempty"`
	Position          int          `json:"position"`
	CreatedAt         int64        `json:"createdAt"`
	StartedAt         *int64       `json:"startedAt,omitempty"`
	CompletedAt       *int64       `json:"completedAt,omitempty"`
}

type SessionRunLock struct {
	SessionID     string `json:"sessionId"`
	RunID         string `json:"runId"`
	ActorClientID string `json:"actorClientId"`
	StartedAt     int64  `json:"startedAt"`
}

type CollabEvent struct {
	ID            string `json:"id"`
	RoomID        string `json:"roomId"`
	SessionID     string `json:"sessionId,omitempty"`
	ActorClientID string `json:"actorClientId,omitempty"`
	Type          string `json:"type"`
	PayloadJSON   string `json:"payloadJson"`
	CreatedAt     int64  `json:"createdAt"`
}

type IdempotencyEntry struct {
	Key        string `json:"key"`
	Scope      string `json:"scope"`
	ResultJSON string `json:"resultJson"`
	CreatedAt  int64  `json:"createdAt"`
}

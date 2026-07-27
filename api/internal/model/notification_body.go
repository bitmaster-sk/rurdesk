package model

// Only types carrying structured body data; comment/mention are excluded
// since message content must not be duplicated into notifications.

type NotificationBodyState struct {
	StateName string `json:"stateName"`
}

type NotificationBodySeverity struct {
	SeverityName  string `json:"severityName"`
	SeverityColor string `json:"severityColor"`
}

// comment/mention carry no body — message content is not duplicated in notifications.
export interface NotificationBodyState {
    stateName: string;
}
export interface NotificationBodySeverity {
    severityName: string;
    severityColor: string;
}

export type NotificationBody = NotificationBodyState | NotificationBodySeverity;

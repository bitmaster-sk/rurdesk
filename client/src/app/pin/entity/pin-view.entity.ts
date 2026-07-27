export interface PinView {
    idPin: number;
    idSeverity?: number;
    severityColor?: string;
    severityName?: string;
    idProject: number;
    idIssuePublic: number;
    title: string;
    stateName?: string | null;
    stateIsStart?: boolean | null;
    stateIsFinal?: boolean | null;
    assignedToName?: string | null;
    assignedToColorAvatarBg?: string | null;
}

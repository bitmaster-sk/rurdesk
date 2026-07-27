import { PinDestinationType } from '../constant/pin-destination-type.enum';

export interface PinIssue {
    idIssue?: number;
    idIssuePublic?: number;
    idProject: number;
    idSeverity: number | null;
    title: string;
    stateName?: string | null;
    stateIsStart?: boolean | null;
    stateIsFinal?: boolean | null;
    assignedToName?: string | null;
    assignedToColorAvatarBg?: string | null;
}

export interface Pin {
    idPin?: number;
    idIssue: number;
    idPinDestination: number;
    idPinDestinationType: PinDestinationType;
    issue?: PinIssue;
}

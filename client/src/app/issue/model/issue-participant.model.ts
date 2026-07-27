export enum ParticipantSource {
    Creator = 'creator',
    Assignee = 'assignee',
    Comment = 'comment',
    Mention = 'mention',
    Manual = 'manual'
}

export interface IssueParticipantModel {
    idUser: number;
    name: string;
    colorAvatarBg: string;
    isBot: boolean;
    source: ParticipantSource;
    hasNotificationsEnabled: boolean;
}

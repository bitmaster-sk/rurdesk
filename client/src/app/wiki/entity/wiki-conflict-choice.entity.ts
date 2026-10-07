export enum WikiConflictPick {
    Mine = 'mine',
    Theirs = 'theirs',
    Both = 'both',
    Custom = 'custom'
}

export interface WikiConflictChoice {
    pick: WikiConflictPick;
    custom: string;
}

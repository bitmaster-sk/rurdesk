export interface IssueState {
    idState: number;
    idProject: number;
    name: string;
    start: boolean;
    final: boolean;
    protected: boolean;
    orderRank: number;
}

export type CreateIssueStateReq = Omit<IssueState, 'idState' | 'protected'>;

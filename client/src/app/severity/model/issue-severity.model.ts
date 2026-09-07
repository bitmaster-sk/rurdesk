export interface IssueSeverity {
    idSeverity: number;
    idProject: number;
    title: string;
    color: string;
    protected: boolean;
    orderRank: number;
}

export type CreateIssueSeverityReq = Omit<IssueSeverity, 'idSeverity' | 'protected'>;

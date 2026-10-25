export interface Tag {
    idTag: number;
    idProject: number;
    name: string;
    color: string;
    orderRank: number;
    issueCount: number;
}

export interface CreateTagReq {
    idProject: number;
    name: string;
    color: string;
}

export interface EditTagReq {
    idTag: number;
    idProject: number;
    name: string;
    color: string;
    orderRank: number;
}

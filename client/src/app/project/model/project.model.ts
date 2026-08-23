export type ProjectInsert = Omit<Project, 'idProject'>;

export interface Project {
    idProject: number;
    name: string;
    color: string;
    idStateDefault?: number | null;
    idSeverityDefault?: number | null;
    idIssueTypeDefault?: number | null;
}

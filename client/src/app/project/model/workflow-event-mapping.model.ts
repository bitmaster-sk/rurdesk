export interface WorkflowEventMapping {
    idProject: number;
    event: string;
    idState: number | null;
}

export interface WorkflowEventMappingEntry {
    event: string;
    idState: number | null;
}

import { IssueViewMode } from '../constants/issue-view-modes.enum';

const STORAGE_KEY_PREFIX = 'rurdesk.issue.lastView';

function key(idProject: number): string {
    return `${STORAGE_KEY_PREFIX}.${idProject}`;
}

export abstract class IssueLastViewStorage {
    public static save(idProject: number, mode: IssueViewMode): void {
        try {
            localStorage.setItem(key(idProject), mode);
        } catch {
            /* best-effort */
        }
    }

    public static load(idProject: number): IssueViewMode | null {
        try {
            const raw = localStorage.getItem(key(idProject));
            return Object.values(IssueViewMode).includes(raw as IssueViewMode)
                ? (raw as IssueViewMode)
                : null;
        } catch {
            return null;
        }
    }
}

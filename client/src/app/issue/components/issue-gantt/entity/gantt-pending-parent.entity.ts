import { Issue } from '../../../model/issue.model';

export interface GanttPendingParent {
    id: number;
    idParent: number;
    placement: { targetId: number; zone: 'before' | 'after' } | null;
    parent: Issue;
    estimated: number;
}

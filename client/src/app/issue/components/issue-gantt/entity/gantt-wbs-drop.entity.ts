import { GanttDropZone } from './gantt-tree.entity';

export type GanttWbsDropKind = GanttDropZone | 'indent' | 'outdent';

export interface GanttWbsDrop {
    movedId: number;
    targetId: number;
    kind: GanttWbsDropKind;
}

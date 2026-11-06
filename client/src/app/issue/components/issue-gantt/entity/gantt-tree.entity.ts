export type ParentMap = Map<number, number>;

export interface GanttTreeRow<T> {
    task: T;
    depth: number;
    idParent: number | null;
    hasChildren: boolean;
    isCollapsed: boolean;
    hiddenCount: number;
    wbsCode: string;
    isContext: boolean;
}

export type GanttDropZone = 'before' | 'after' | 'into';

export interface GanttOutdentTarget {
    idParent: number | null;
    afterId: number;
}

export interface GanttVisibleRowsOptions {
    collapsed: ReadonlySet<number>;
    outlineLevel: number | null;
    contextIds?: ReadonlySet<number>;
}

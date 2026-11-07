import { CdkDragMove } from '@angular/cdk/drag-drop';
import { GanttWbsDrop } from '../../../entity/gantt-wbs-drop.entity';

export type GanttRowDragMove = Pick<CdkDragMove<unknown>, 'pointerPosition' | 'distance'> & {
    source: Pick<CdkDragMove<unknown>['source'], 'getPlaceholderElement'>;
};

export interface GanttWbsDropTarget extends GanttWbsDrop {
    isAllowed: boolean;
}

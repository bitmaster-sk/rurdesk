import { ChangeDetectionStrategy, Component, input, output, signal } from '@angular/core';
import { Issue } from '../../../../model/issue.model';
import { IssueRelationType } from '../../../../constants/issue-relation-type.enum';
import { IssueRelationSubType } from '../../../../constants/issue-relation-subtype.enum';

export interface RelationDropEvent {
    toIssue: Issue;
    relationType: IssueRelationType;
    subType: IssueRelationSubType | null;
}

@Component({
    selector: 'app-issue-table-drop-zone',
    templateUrl: './issue-table-drop-zone.component.html',
    styleUrls: ['./issue-table-drop-zone.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueTableDropZoneComponent {
    public readonly targetIssue = input.required<Issue>();

    // Fires when a pill is dropped — parent creates the relation
    public readonly relationDrop = output<RelationDropEvent>();

    // Fires on dragenter into any pill — parent uses this to cancel its drag-leave timer
    public readonly zoneEnter = output<void>();

    public readonly rowBasic = [
        {
            relationType: IssueRelationType.Duplicates,
            subType: null,
            labelKey: 'RELATION.DUPLICATES'
        },
        {
            relationType: IssueRelationType.RelatesTo,
            subType: null,
            labelKey: 'RELATION.RELATES_TO'
        }
    ];

    public readonly rowHierarchy = [
        {
            relationType: IssueRelationType.Hierarchy,
            subType: IssueRelationSubType.Parent,
            labelKey: 'RELATION.PARENT'
        },
        {
            relationType: IssueRelationType.Hierarchy,
            subType: IssueRelationSubType.Child,
            labelKey: 'RELATION.CHILD'
        }
    ];

    public readonly rowSchedule = [
        {
            relationType: IssueRelationType.Schedule,
            subType: IssueRelationSubType.FinishToStart,
            labelKey: 'RELATION.FINISH_TO_START'
        },
        {
            relationType: IssueRelationType.Schedule,
            subType: IssueRelationSubType.StartToStart,
            labelKey: 'RELATION.START_TO_START'
        },
        {
            relationType: IssueRelationType.Schedule,
            subType: IssueRelationSubType.FinishToFinish,
            labelKey: 'RELATION.FINISH_TO_FINISH'
        },
        {
            relationType: IssueRelationType.Schedule,
            subType: IssueRelationSubType.StartToFinish,
            labelKey: 'RELATION.START_TO_FINISH'
        }
    ];

    // Key of the pill currently under the drag cursor: "relationType:subType"
    public hoveredPillKey = signal<string | null>(null);

    public pillKey(relationType: IssueRelationType, subType: IssueRelationSubType | null): string {
        return `${relationType}:${subType}`;
    }

    public onPillDragEnter(
        relationType: IssueRelationType,
        subType: IssueRelationSubType | null
    ): void {
        this.hoveredPillKey.set(this.pillKey(relationType, subType));
        this.zoneEnter.emit();
    }

    public onPillDragLeave(
        relationType: IssueRelationType,
        subType: IssueRelationSubType | null
    ): void {
        if (this.hoveredPillKey() === this.pillKey(relationType, subType)) {
            this.hoveredPillKey.set(null);
        }
    }

    public onDrop(
        event: DragEvent,
        relationType: IssueRelationType,
        subType: IssueRelationSubType | null
    ): void {
        event.preventDefault();
        event.stopPropagation();
        this.hoveredPillKey.set(null);
        this.relationDrop.emit({ toIssue: this.targetIssue(), relationType, subType });
    }
}

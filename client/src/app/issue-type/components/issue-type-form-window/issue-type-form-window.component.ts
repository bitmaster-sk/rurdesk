import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { Project } from 'src/app/project/model/project.model';
import { WindowConfig } from 'src/app/shared/window/entity/window-config';
import { WindowReference } from 'src/app/shared/window/window.reference';
import { IssueType } from '../../model/issue-type.model';
import { IssueTypeApi } from '../../api/issue-type.api.service';

export interface IssueTypeWindowData {
    project?: Project;
    issueType?: IssueType;
}

@Component({
    selector: 'app-issue-type-form-window',
    templateUrl: './issue-type-form-window.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class IssueTypeFormWindowComponent {
    private winRef = inject(WindowReference);
    public winCfg = inject<WindowConfig<IssueTypeWindowData>>(WindowConfig);
    private issueTypeApi = inject(IssueTypeApi);

    public onSave(issueType: IssueType): void {
        const saver = issueType.idIssueType
            ? this.issueTypeApi.update$(issueType)
            : this.issueTypeApi.insert$(issueType);
        saver.subscribe(savedIssueType => this.winRef.close(savedIssueType));
    }

    public onCancel(): void {
        this.winRef.close(null);
    }

    public get issueType(): Partial<IssueType> {
        return {
            idProject: this.winCfg.data?.project?.idProject,
            ...this.winCfg.data?.issueType
        };
    }
}

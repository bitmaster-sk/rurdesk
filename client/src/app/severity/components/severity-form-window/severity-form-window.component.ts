import { Component, inject } from '@angular/core';
import { Project } from 'src/app/project/model/project.model';
import { WindowConfig } from 'src/app/shared/window/entity/window-config';
import { WindowReference } from 'src/app/shared/window/window.reference';
import { IssueSeverity } from '../../model/issue-severity.model';
import { SeverityApi } from '../../api/severity.api.service';

export interface SeverityWindowData {
    project?: Project;
    severity?: IssueSeverity;
}

@Component({
    selector: 'app-severity-form-window',
    templateUrl: './severity-form-window.component.html',
    standalone: false
})
export class SeverityFormWindowComponent {
    private winRef = inject(WindowReference);
    public winCfg = inject<WindowConfig<SeverityWindowData>>(WindowConfig);
    private severityApi = inject(SeverityApi);

    public onSave(severity: IssueSeverity): void {
        const saver = severity.idSeverity
            ? this.severityApi.update$(severity)
            : this.severityApi.insert$(severity);
        saver.subscribe(savedSeverity => this.winRef.close(savedSeverity));
    }

    public onCancel(): void {
        this.winRef.close(null);
    }

    public get severity(): IssueSeverity {
        return {
            idProject: this.winCfg.data?.['project']?.idProject,
            ...this.winCfg.data?.['severity']
        };
    }
}

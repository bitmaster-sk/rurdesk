import { Component, inject } from '@angular/core';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { Project } from 'src/app/project/model/project.model';
import { WindowConfig } from 'src/app/shared/window/entity/window-config';
import { WindowReference } from 'src/app/shared/window/window.reference';
import { StateApi } from '../../api/state.api.service';

export interface StateWindowData {
    project?: Project;
    state?: IssueState;
}

@Component({
    selector: 'app-state-form-window',
    templateUrl: './state-form-window.component.html',
    standalone: false
})
export class StateFormWindowComponent {
    private winRef = inject(WindowReference);
    public winCfg = inject<WindowConfig<StateWindowData>>(WindowConfig);
    private stateApi = inject(StateApi);

    public onSave(state: IssueState): void {
        const saver = state.idState ? this.stateApi.update$(state) : this.stateApi.insert$(state);
        saver.subscribe(savedState => this.winRef.close(savedState));
    }

    public onCancel(): void {
        this.winRef.close(null);
    }

    public get state(): IssueState {
        return {
            idProject: this.winCfg.data?.['project']?.idProject,
            ...this.winCfg.data?.['state']
        };
    }
}

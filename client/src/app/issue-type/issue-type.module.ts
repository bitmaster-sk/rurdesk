import { NgModule } from '@angular/core';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { DragDropModule } from '@angular/cdk/drag-drop';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconPlus,
    IconGripVertical,
    IconPencil,
    IconTrash,
    IconDeviceFloppy,
    IconCancel
} from '@tabler/icons-angular';

import { CoreModule } from '../core/core.module';
import { SharedModule } from '../shared/shared.module';
import { UiModule } from '../ui/ui.module';
import { IssueTypeFormComponent } from './components/issue-type-form/issue-type-form.component';
import { IssueTypeFormWindowComponent } from './components/issue-type-form-window/issue-type-form-window.component';
import { ProjectIssueTypeComponent } from './components/project-issue-type/project-issue-type.component';

@NgModule({
    declarations: [IssueTypeFormComponent, IssueTypeFormWindowComponent, ProjectIssueTypeComponent],
    imports: [
        CoreModule,
        FormsModule,
        SharedModule,
        ReactiveFormsModule,
        DragDropModule,
        UiModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconPlus,
            IconGripVertical,
            IconPencil,
            IconTrash,
            IconDeviceFloppy,
            IconCancel
        })
    ],
    exports: [ProjectIssueTypeComponent]
})
export class IssueTypeModule {}

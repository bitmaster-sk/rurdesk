import { NgModule } from '@angular/core';

import { SeverityFormComponent } from './components/severity-form/severity-form.component';
import { SeverityFormWindowComponent } from './components/severity-form-window/severity-form-window.component';
import { ProjectSeverityComponent } from './components/project-severity/project-severity.component';
import { SeverityCircleComponent } from './components/severity-circle/severity-circle.component';
import { CoreModule } from '../core/core.module';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { DragDropModule } from '@angular/cdk/drag-drop';
import { UiModule } from '../ui/ui.module';
import { SharedModule } from '../shared/shared.module';
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

@NgModule({
    declarations: [
        SeverityFormComponent,
        SeverityFormWindowComponent,
        ProjectSeverityComponent,
        SeverityCircleComponent
    ],
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
    exports: [ProjectSeverityComponent, SeverityCircleComponent]
})
export class SeverityModule {}

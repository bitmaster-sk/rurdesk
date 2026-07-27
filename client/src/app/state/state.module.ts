import { NgModule } from '@angular/core';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { CoreModule } from '../core/core.module';
import { StateFormComponent } from './components/state-form/state-form.component';
import { StateFormWindowComponent } from './components/state-form-window/state-form-window.component';
import { ProjectStateComponent } from './components/project-state/project-state.component';
import { DragDropModule } from '@angular/cdk/drag-drop';
import { UiModule } from '../ui/ui.module';
import { SharedModule } from '../shared/shared.module';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconPlus,
    IconGripVertical,
    IconCircleCheck,
    IconCircleX,
    IconPencil,
    IconTrash,
    IconDeviceFloppy,
    IconCancel
} from '@tabler/icons-angular';

@NgModule({
    declarations: [StateFormComponent, StateFormWindowComponent, ProjectStateComponent],
    imports: [
        CoreModule,
        FormsModule,
        ReactiveFormsModule,
        DragDropModule,
        UiModule,
        SharedModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconPlus,
            IconGripVertical,
            IconCircleCheck,
            IconCircleX,
            IconPencil,
            IconTrash,
            IconDeviceFloppy,
            IconCancel
        })
    ],
    exports: [ProjectStateComponent]
})
export class StateModule {}

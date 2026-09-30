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
    IconCancel,
    IconArchive,
    IconArchiveOff
} from '@tabler/icons-angular';

import { CoreModule } from '../core/core.module';
import { SharedModule } from '../shared/shared.module';
import { UiModule } from '../ui/ui.module';
import { CustomFieldFormComponent } from './components/custom-field-form/custom-field-form.component';
import { CustomFieldValueInputComponent } from './components/custom-field-value-input/custom-field-value-input.component';
import { CustomFieldFormWindowComponent } from './components/custom-field-form-window/custom-field-form-window.component';
import { ProjectCustomFieldComponent } from './components/project-custom-field/project-custom-field.component';

@NgModule({
    declarations: [
        CustomFieldFormComponent,
        CustomFieldFormWindowComponent,
        CustomFieldValueInputComponent,
        ProjectCustomFieldComponent
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
            IconCancel,
            IconArchive,
            IconArchiveOff
        })
    ],
    exports: [ProjectCustomFieldComponent]
})
export class CustomFieldModule {}

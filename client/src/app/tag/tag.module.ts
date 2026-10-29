import { NgModule } from '@angular/core';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { DragDropModule } from '@angular/cdk/drag-drop';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconGripVertical,
    IconPencil,
    IconTrash,
    IconDeviceFloppy,
    IconCancel
} from '@tabler/icons-angular';
import { CoreModule } from '../core/core.module';
import { SharedModule } from '../shared/shared.module';
import { UiModule } from '../ui/ui.module';
import { ProjectTagComponent } from './components/project-tag/project-tag.component';
import { TagFormComponent } from './components/tag-form/tag-form.component';
import { TagFormWindowComponent } from './components/tag-form-window/tag-form-window.component';

@NgModule({
    declarations: [ProjectTagComponent, TagFormComponent, TagFormWindowComponent],
    imports: [
        CoreModule,
        FormsModule,
        ReactiveFormsModule,
        SharedModule,
        DragDropModule,
        UiModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconGripVertical,
            IconPencil,
            IconTrash,
            IconDeviceFloppy,
            IconCancel
        })
    ],
    exports: [ProjectTagComponent]
})
export class TagModule {}

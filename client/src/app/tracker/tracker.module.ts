import { NgModule } from '@angular/core';
import { TrackTableComponent } from './track-table/track-table.component';
import { SharedModule } from '../shared/shared.module';
import { UiModule } from '../ui/ui.module';
import { CoreModule } from '../core/core.module';
import { TrackFormComponent } from './track-form/track-form.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconRotateClockwise,
    IconPencil,
    IconTrash
} from '@tabler/icons-angular';

@NgModule({
    declarations: [TrackTableComponent, TrackFormComponent],
    imports: [
        CoreModule,
        SharedModule,
        UiModule,
        FormsModule,
        ReactiveFormsModule,
        TablerIconComponent
    ],
    providers: [provideTablerIcons({ IconRotateClockwise, IconPencil, IconTrash })],
    exports: [TrackTableComponent]
})
export class TrackerModule {}

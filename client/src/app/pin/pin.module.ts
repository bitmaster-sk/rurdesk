import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CoreModule } from '../core/core.module';
import { RouterModule } from '@angular/router';
import { SharedModule } from '../shared/shared.module';
import { PinViewComponent } from './pin-view/pin-view.component';
import { TablerIconComponent, provideTablerIcons, IconX } from '@tabler/icons-angular';

@NgModule({
    declarations: [PinViewComponent],
    imports: [CommonModule, CoreModule, RouterModule, SharedModule, TablerIconComponent],
    providers: [provideTablerIcons({ IconX })],
    exports: [PinViewComponent]
})
export class PinModule {}

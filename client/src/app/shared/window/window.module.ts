import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WindowComponent } from './window/window.component';
import { UiModule } from '../../ui/ui.module';

@NgModule({
    declarations: [WindowComponent],
    imports: [CommonModule, UiModule]
})
export class WindowModule {}

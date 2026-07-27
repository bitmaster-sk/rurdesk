import { CommonModule } from '@angular/common';
import { NgModule } from '@angular/core';
import {
    IconAlertCircle,
    IconCircleCheck,
    IconInfoCircle,
    IconX,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { CoreModule } from '../core/core.module';
import { UiToastComponent } from './components/toast/toast.component';

/**
 * Thin, eager-friendly module for the root toast host. Kept separate from
 * `UiModule` so importing it into `AppModule` does not drag the whole ui-*
 * family (+ CDK OverlayModule) into the initial bundle. The toast icons are
 * registered HERE (pattern: pin.module) — registering them in the lazy-only
 * `UiModule` would leave them unresolved in the eager root injector.
 */
@NgModule({
    declarations: [UiToastComponent],
    imports: [CommonModule, CoreModule, TablerIconComponent],
    exports: [UiToastComponent],
    providers: [provideTablerIcons({ IconCircleCheck, IconInfoCircle, IconAlertCircle, IconX })]
})
export class UiToastModule {}

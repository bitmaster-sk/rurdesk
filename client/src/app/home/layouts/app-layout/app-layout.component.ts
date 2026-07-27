import { ChangeDetectionStrategy, Component } from '@angular/core';

@Component({
    selector: 'app-app-layout',
    templateUrl: './app-layout.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AppLayoutComponent {}

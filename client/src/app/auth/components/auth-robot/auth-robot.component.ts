import { ChangeDetectionStrategy, Component } from '@angular/core';

@Component({
    selector: 'app-auth-robot',
    templateUrl: './auth-robot.component.html',
    styleUrls: ['./auth-robot.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class AuthRobotComponent {}

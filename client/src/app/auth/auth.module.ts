import { NgModule } from '@angular/core';
import { AuthPage } from './auth/auth.page';
import { AuthRobotComponent } from './components/auth-robot/auth-robot.component';
import { LoginComponent } from './login/login.component';
import { RegisterComponent } from './register/register.component';
import { SsoCallbackPage } from './sso-callback/sso-callback.page';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import {
    IconBuilding,
    IconEye,
    IconEyeOff,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { CoreModule } from '../core/core.module';
import { UiModule } from '../ui/ui.module';

@NgModule({
    declarations: [
        AuthPage,
        AuthRobotComponent,
        LoginComponent,
        RegisterComponent,
        SsoCallbackPage
    ],
    imports: [CoreModule, UiModule, ReactiveFormsModule, RouterModule, TablerIconComponent],
    exports: [AuthPage, SsoCallbackPage],
    providers: [provideTablerIcons({ IconBuilding, IconEye, IconEyeOff })]
})
export class AuthModule {}

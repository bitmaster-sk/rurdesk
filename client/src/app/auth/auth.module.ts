import { NgModule } from '@angular/core';
import { AuthPage } from './auth/auth.page';
import { AuthRobotComponent } from './components/auth-robot/auth-robot.component';
import { LoginComponent } from './login/login.component';
import { RegisterComponent } from './register/register.component';
import { ReactiveFormsModule } from '@angular/forms';
import {
    IconEye,
    IconEyeOff,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { CoreModule } from '../core/core.module';
import { UiModule } from '../ui/ui.module';

@NgModule({
    declarations: [AuthPage, AuthRobotComponent, LoginComponent, RegisterComponent],
    imports: [CoreModule, UiModule, ReactiveFormsModule, TablerIconComponent],
    exports: [AuthPage],
    providers: [provideTablerIcons({ IconEye, IconEyeOff })]
})
export class AuthModule {}

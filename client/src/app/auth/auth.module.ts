import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AuthPage } from './auth/auth.page';
import { AuthRobotComponent } from './components/auth-robot/auth-robot.component';
import { LoginComponent } from './login/login.component';
import { RegisterComponent } from './register/register.component';
import { ReactiveFormsModule } from '@angular/forms';
import { CoreModule } from '../core/core.module';
import { UiModule } from '../ui/ui.module';

@NgModule({
    declarations: [AuthPage, AuthRobotComponent, LoginComponent, RegisterComponent],
    imports: [CoreModule, UiModule, ReactiveFormsModule],
    exports: [AuthPage]
})
export class AuthModule {}

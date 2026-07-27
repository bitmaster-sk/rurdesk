import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconSparkles,
    IconDeviceFloppy
} from '@tabler/icons-angular';
import { OnboardingRoutingModule } from './onboarding-routing.module';
import { OnboardingFirstProjectComponent } from './pages/onboarding-first-project/onboarding-first-project.component';
import { UiModule } from '../ui/ui.module';

@NgModule({
    declarations: [OnboardingFirstProjectComponent],
    imports: [
        CommonModule,
        ReactiveFormsModule,
        TranslateModule,
        TablerIconComponent,
        OnboardingRoutingModule,
        UiModule
    ],
    providers: [provideTablerIcons({ IconSparkles, IconDeviceFloppy })]
})
export class OnboardingModule {}

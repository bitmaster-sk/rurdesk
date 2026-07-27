import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { OnboardingFirstProjectComponent } from './pages/onboarding-first-project/onboarding-first-project.component';

const routes: Routes = [{ path: '', component: OnboardingFirstProjectComponent }];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class OnboardingRoutingModule {}

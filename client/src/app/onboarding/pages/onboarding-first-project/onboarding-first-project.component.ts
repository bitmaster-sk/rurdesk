import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormControl, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { ProjectService } from '../../../project/project.service';
import { Project } from '../../../project/model/project.model';
import { ToastNotificationService } from '../../../core/toast-notification.service';

@Component({
    selector: 'app-onboarding-first-project',
    templateUrl: './onboarding-first-project.component.html',
    styleUrls: ['./onboarding-first-project.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class OnboardingFirstProjectComponent {
    private readonly sProject = inject(ProjectService);
    private readonly router = inject(Router);
    private readonly toast = inject(ToastNotificationService);

    public readonly name = new FormControl<string>('', {
        nonNullable: true,
        validators: [Validators.required, Validators.maxLength(250)]
    });

    // True while a create request is in flight — disables both actions so a
    // double-click can't create two projects before navigation resolves.
    public readonly isSubmitting = signal(false);

    public onCreateBlank(): void {
        this.create(idProject => this.router.navigate(['/project', idProject, 'view']));
    }

    public onCreateWithAI(): void {
        this.create(idProject => this.router.navigate(['/project', idProject, 'project-builder']));
    }

    private create(then: (idProject: number) => void): void {
        if (this.name.invalid || this.isSubmitting()) {
            this.name.markAsTouched();
            return;
        }
        const name = this.name.value.trim();
        if (!name) {
            this.name.markAsTouched();
            return;
        }
        this.isSubmitting.set(true);
        // Project requires name + color (idProject/defaults optional). Empty color
        // matches the existing "+" dialog create — the API treats color as omitempty.
        const project: Project = { name, color: '' };
        this.sProject.insertProject(project).subscribe({
            next: saved => then(saved.idProject),
            error: () => {
                this.isSubmitting.set(false);
                this.toast.showError('ONBOARDING.CREATE_FAILED');
            }
        });
    }
}

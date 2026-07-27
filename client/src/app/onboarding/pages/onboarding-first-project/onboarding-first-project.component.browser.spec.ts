import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { of, throwError, Subject } from 'rxjs';
import { TranslateModule } from '@ngx-translate/core';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconSparkles,
    IconDeviceFloppy
} from '@tabler/icons-angular';
import { OnboardingFirstProjectComponent } from './onboarding-first-project.component';
import { ProjectService } from '../../../project/project.service';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { UiModule } from '../../../ui/ui.module';

describe('OnboardingFirstProjectComponent', () => {
    const insertProject = vi.fn();
    const navigate = vi.fn();
    const showError = vi.fn();

    beforeEach(async () => {
        insertProject.mockReset();
        navigate.mockReset();
        showError.mockReset();
        insertProject.mockReturnValue(of({ idProject: 7, name: 'Website relaunch', color: '' }));

        // Logic-focused spec: blank the template so we don't need
        // TablerIconComponent just to instantiate. async + compileComponents()
        // matches the existing template-blanking specs (templateUrl/styleUrls need it).
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot()],
            declarations: [OnboardingFirstProjectComponent],
            providers: [
                { provide: ProjectService, useValue: { insertProject } },
                { provide: Router, useValue: { navigate } },
                { provide: ToastNotificationService, useValue: { showError } }
            ]
        })
            .overrideComponent(OnboardingFirstProjectComponent, { set: { template: '' } })
            .compileComponents();
    });

    function make(): OnboardingFirstProjectComponent {
        const fixture = TestBed.createComponent(OnboardingFirstProjectComponent);
        return fixture.componentInstance;
    }

    it('does not create while the name is empty', () => {
        const c = make();
        c.onCreateWithAI();
        expect(insertProject).not.toHaveBeenCalled();
        expect(navigate).not.toHaveBeenCalled();
    });

    it('AI path creates the project then opens the builder', () => {
        const c = make();
        c.name.setValue('Website relaunch');
        c.onCreateWithAI();
        expect(insertProject).toHaveBeenCalledWith({ name: 'Website relaunch', color: '' });
        expect(navigate).toHaveBeenCalledWith(['/project', 7, 'project-builder']);
    });

    it('blank path creates the project then opens the board', () => {
        const c = make();
        c.name.setValue('Website relaunch');
        c.onCreateBlank();
        expect(insertProject).toHaveBeenCalledWith({ name: 'Website relaunch', color: '' });
        expect(navigate).toHaveBeenCalledWith(['/project', 7, 'view']);
    });

    it('trims the project name before sending', () => {
        const c = make();
        c.name.setValue('  Website relaunch  ');
        c.onCreateBlank();
        expect(insertProject).toHaveBeenCalledWith({ name: 'Website relaunch', color: '' });
    });

    it('shows an error toast and does not navigate when create fails', () => {
        insertProject.mockReturnValue(throwError(() => new Error('boom')));
        const c = make();
        c.name.setValue('Website relaunch');
        c.onCreateWithAI();
        expect(navigate).not.toHaveBeenCalled();
        expect(showError).toHaveBeenCalledWith('ONBOARDING.CREATE_FAILED');
        expect(c.isSubmitting()).toBe(false);
    });

    it('ignores a second submit while the first request is in flight', () => {
        insertProject.mockReturnValue(new Subject()); // never completes
        const c = make();
        c.name.setValue('Website relaunch');
        c.onCreateWithAI();
        c.onCreateBlank();
        expect(insertProject).toHaveBeenCalledTimes(1);
    });
});

// Separate suite: renders the REAL template (UiModule imported) to guard that
// the name field actually receives the ui-input chrome — a regression guard for
// the module forgetting to import UiModule, which left `uiInput` inert.
describe('OnboardingFirstProjectComponent (template)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                TranslateModule.forRoot(),
                ReactiveFormsModule,
                UiModule,
                TablerIconComponent
            ],
            declarations: [OnboardingFirstProjectComponent],
            providers: [
                provideTablerIcons({ IconSparkles, IconDeviceFloppy }),
                { provide: ProjectService, useValue: { insertProject: vi.fn() } },
                { provide: Router, useValue: { navigate: vi.fn() } },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } }
            ]
        }).compileComponents();
    });

    it('applies the ui-input directive to the name field', () => {
        const fixture = TestBed.createComponent(OnboardingFirstProjectComponent);
        fixture.detectChanges();
        const input = fixture.nativeElement.querySelector('#ob-name') as HTMLInputElement;
        expect(input).not.toBeNull();
        expect(input.classList.contains('ui-input')).toBe(true);
    });
});

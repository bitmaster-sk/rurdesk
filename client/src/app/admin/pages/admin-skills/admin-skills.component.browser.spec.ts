import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { Fixtures } from 'src/testing/fixtures';
import { TablerIconStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { SkillApi } from '../../../shared/api/skill.api.service';
import { Skill } from '../../../shared/model/skill.model';
import { UiModule } from '../../../ui/ui.module';
import { AdminSkillsComponent } from './admin-skills.component';

const BUILTIN: Skill = Fixtures.skill({ idSkill: 1, name: 'Verification rules' });
const EDITED: Skill = Fixtures.skill({
    idSkill: 2,
    name: 'Repository rules',
    description: 'agents.md',
    isEdited: true
});
const CUSTOM: Skill = Fixtures.skill({
    idSkill: 3,
    name: 'House style',
    description: 'ours',
    isBuiltin: false
});

abstract class Dom {
    public static rows(root: HTMLElement): HTMLElement[] {
        return Array.from(root.querySelectorAll('[data-testid="skill-row"]'));
    }

    public static badges(root: HTMLElement): (string | undefined)[] {
        return Array.from(root.querySelectorAll('ui-badge')).map(el => el.textContent?.trim());
    }

    public static byTestId(root: HTMLElement, testId: string): HTMLElement | null {
        return root.querySelector(`[data-testid="${testId}"]`);
    }

    // uiConfirm renders its Yes/No panel in a body-level overlay, outside the fixture.
    public static confirmAccept(): void {
        const buttons = Array.from(
            document.querySelectorAll<HTMLElement>('.ui-confirm-panel button')
        );
        buttons[buttons.length - 1].click();
    }
}

describe('AdminSkillsComponent (browser)', () => {
    let api: {
        load$: ReturnType<typeof vi.fn>;
        create$: ReturnType<typeof vi.fn>;
        update$: ReturnType<typeof vi.fn>;
        delete$: ReturnType<typeof vi.fn>;
        restore$: ReturnType<typeof vi.fn>;
    };
    let toast: { showError: ReturnType<typeof vi.fn> };

    beforeEach(() => {
        api = {
            load$: vi.fn().mockReturnValue(of([BUILTIN, EDITED, CUSTOM])),
            create$: vi.fn().mockReturnValue(of(CUSTOM)),
            update$: vi
                .fn()
                .mockImplementation((_idSkill: number, body: Record<string, string>) =>
                    of({ ...BUILTIN, ...body })
                ),
            delete$: vi.fn().mockReturnValue(of(undefined)),
            restore$: vi.fn().mockReturnValue(of(BUILTIN))
        };
        toast = { showError: vi.fn() };
    });

    async function setup() {
        await TestBed.configureTestingModule({
            imports: [UiModule, ReactiveFormsModule, TranslateModule.forRoot(), TablerIconStub],
            declarations: [AdminSkillsComponent],
            providers: [
                { provide: SkillApi, useValue: api },
                { provide: ToastNotificationService, useValue: toast }
            ]
        }).compileComponents();

        const fixture = TestBed.createComponent(AdminSkillsComponent);
        fixture.detectChanges();
        return fixture;
    }

    it('badges each catalog row as builtin, edited or custom', async () => {
        const fixture = await setup();

        expect(Dom.badges(fixture.nativeElement)).toEqual([
            'SKILL.BADGE.BUILTIN',
            'SKILL.BADGE.EDITED',
            'SKILL.BADGE.CUSTOM'
        ]);
    });

    it('offers restore on a builtin and delete on a custom skill', async () => {
        const fixture = await setup();
        const root = fixture.nativeElement as HTMLElement;

        Dom.rows(root)[0].click();
        fixture.detectChanges();
        expect(Dom.byTestId(root, 'skill-restore')).toBeTruthy();
        expect(Dom.byTestId(root, 'skill-delete')).toBeNull();

        Dom.rows(root)[2].click();
        fixture.detectChanges();
        expect(Dom.byTestId(root, 'skill-delete')).toBeTruthy();
        expect(Dom.byTestId(root, 'skill-restore')).toBeNull();
    });

    it('typing in the content editor patches only that field, once', async () => {
        vi.useFakeTimers();
        try {
            const fixture = await setup();
            const root = fixture.nativeElement as HTMLElement;
            Dom.rows(root)[0].click();
            fixture.detectChanges();

            const textarea = root.querySelector('#skillContent') as HTMLTextAreaElement;
            textarea.value = 'changed';
            textarea.dispatchEvent(new Event('input'));
            vi.advanceTimersByTime(700);
            fixture.detectChanges();

            expect(api.update$).toHaveBeenCalledTimes(1);
            expect(api.update$).toHaveBeenCalledWith(1, { content: 'changed' });
        } finally {
            vi.useRealTimers();
        }
    });

    it('a failed save toasts', async () => {
        vi.useFakeTimers();
        try {
            api.update$ = vi.fn().mockReturnValue(throwError(() => new Error('nope')));
            const fixture = await setup();
            const root = fixture.nativeElement as HTMLElement;
            Dom.rows(root)[0].click();
            fixture.detectChanges();

            const nameInput = root.querySelector('#skillName') as HTMLInputElement;
            nameInput.value = 'renamed';
            nameInput.dispatchEvent(new Event('input'));
            vi.advanceTimersByTime(700);
            fixture.detectChanges();

            expect(toast.showError).toHaveBeenCalledWith('SKILL.SAVE_ERROR');
        } finally {
            vi.useRealTimers();
        }
    });

    it('restore runs only after the confirmation is accepted', async () => {
        const fixture = await setup();
        const root = fixture.nativeElement as HTMLElement;
        Dom.rows(root)[0].click();
        fixture.detectChanges();

        Dom.byTestId(root, 'skill-restore')!.click();
        fixture.detectChanges();
        expect(api.restore$).not.toHaveBeenCalled();

        Dom.confirmAccept();
        fixture.detectChanges();

        expect(api.restore$).toHaveBeenCalledWith(1);
        expect(api.load$).toHaveBeenCalledTimes(2);
    });

    it('deleting a custom skill clears the editor', async () => {
        const fixture = await setup();
        const root = fixture.nativeElement as HTMLElement;
        Dom.rows(root)[2].click();
        fixture.detectChanges();

        Dom.byTestId(root, 'skill-delete')!.click();
        fixture.detectChanges();
        Dom.confirmAccept();
        fixture.detectChanges();

        expect(api.delete$).toHaveBeenCalledWith(3);
        expect(Dom.byTestId(root, 'skill-editor')).toBeNull();
    });
});

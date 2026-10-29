import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { RouterLink, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { TablerIconStub, UiLoaderStub } from 'src/testing/stubs';
import { describe, expect, it } from 'vitest';
import { ReleaseOpenIssues } from '../../constants/release-open-issues.enum';
import { ReleaseDraft } from '../../model/release.model';
import { ReleaseIssueListComponent } from '../release-issue-list/release-issue-list.component';
import { ReleaseTasksStepComponent } from './release-tasks-step.component';

const DRAFT: ReleaseDraft = {
    sections: [
        {
            idIssueType: 2,
            name: 'Feature',
            issues: [
                {
                    idIssue: 20,
                    idIssuePublic: 104,
                    title: 'Release notes',
                    idIssueType: 2,
                    issueTypeName: 'Feature'
                }
            ]
        }
    ],
    openIssues: [
        {
            idIssue: 21,
            idIssuePublic: 105,
            title: 'Half done',
            idIssueType: 2,
            issueTypeName: 'Feature'
        }
    ]
};

function render(draft: ReleaseDraft | null) {
    TestBed.configureTestingModule({
        imports: [
            TranslateModule.forRoot(),
            ReactiveFormsModule,
            RouterLink,
            TablerIconStub,
            UiLoaderStub
        ],
        declarations: [ReleaseTasksStepComponent, ReleaseIssueListComponent],
        providers: [provideRouter([])]
    });
    const fixture = TestBed.createComponent(ReleaseTasksStepComponent);
    const openIssues = new FormControl(ReleaseOpenIssues.Move, { nonNullable: true });
    fixture.componentRef.setInput('idProject', 3);
    fixture.componentRef.setInput('draft', draft);
    fixture.componentRef.setInput('tagName', 'next');
    fixture.componentRef.setInput('openIssues', openIssues);
    fixture.detectChanges();
    return { element: fixture.nativeElement as HTMLElement, openIssues };
}

describe('ReleaseTasksStepComponent', () => {
    it('says what it loads while the tasks are on their way', () => {
        const { element } = render(null);

        expect(element.querySelector('[role="status"]')?.textContent).toContain(
            'RELEASE.TASKS_STEP.LOADING'
        );
    });

    it('links every task to its detail in a new tab', () => {
        const { element } = render(DRAFT);

        const links = Array.from(
            element.querySelectorAll<HTMLAnchorElement>('[data-testid="release-issue"]')
        );
        expect(
            links.map(link => link.querySelector('.release-issue__title')?.textContent?.trim())
        ).toEqual(['Release notes', 'Half done']);
        expect(links[0].getAttribute('href')).toBe('/project/3/issue/104');
        expect(links[0].target).toBe('_blank');
    });

    it('lets the user keep the open tasks in the release', () => {
        const { element, openIssues } = render(DRAFT);

        const keep = element.querySelector<HTMLInputElement>('[data-testid="release-open-keep"]')!;
        const move = element.querySelector<HTMLInputElement>('[data-testid="release-open-move"]')!;
        expect(move.checked).toBe(true);
        keep.click();

        expect(openIssues.value).toBe(ReleaseOpenIssues.Keep);
    });

    it('shows a single warning when the tag has only open tasks', () => {
        const { element } = render({ sections: [], openIssues: DRAFT.openIssues });

        const warnings = element.querySelectorAll('[data-testid="release-tasks-warning"]');
        expect(warnings).toHaveLength(1);
        expect(warnings[0].textContent).toContain('RELEASE.TASKS_STEP.EMPTY');
        expect(warnings[0].textContent).toContain('RELEASE.TASKS_STEP.EMPTY_OPEN_HINT');
        expect(element.querySelector('[data-testid="release-open-move"]')).not.toBeNull();
    });

    it('warns about a tag without tasks but does not block the release', () => {
        const { element } = render({ sections: [], openIssues: [] });

        const warning = element.querySelector('[data-testid="release-tasks-warning"]');
        expect(warning?.textContent).toContain('RELEASE.TASKS_STEP.EMPTY_HINT');
        expect(element.querySelector('[data-testid="release-open-move"]')).toBeNull();
    });
});

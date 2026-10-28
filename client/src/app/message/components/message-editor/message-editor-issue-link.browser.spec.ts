import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { Subject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { Issue } from 'src/app/issue/model/issue.model';
import { Project } from 'src/app/project/model/project.model';
import { LinkScope } from 'src/app/link/service/link-scope.service';
import { MessageModule } from '../../message.module';
import { EditorText } from './editor-text';

@Component({
    standalone: false,
    template: '<app-message-editor></app-message-editor>'
})
class HostComponent {}

const issue = (idIssuePublic: number, title: string): Issue => {
    return {
        idIssue: idIssuePublic,
        idIssuePublic,
        idProject: 7,
        idState: null,
        idSeverity: null,
        idIssueType: null,
        title,
        description: '',
        tracked: 0
    };
};

const project = (idProject: number, name: string): Project => ({
    idProject,
    name,
    color: ''
});

const issues: Issue[] = [issue(123, 'Fix login'), issue(456, 'Add dark mode')];

const projects: Project[] = [project(7, 'Rurdesk'), project(9, 'Website')];

async function render(options: { idProject?: number | null; withScope?: boolean }): Promise<{
    fixture: ComponentFixture<HostComponent>;
    load: ReturnType<typeof vi.fn>;
    loadProjects: ReturnType<typeof vi.fn>;
}> {
    const load = vi.fn().mockImplementation(() => new Subject<Issue[]>());
    const loadProjects = vi.fn().mockImplementation(() => new Subject<Project[]>());
    const providers: object[] = [
        { provide: IssueApi, useValue: { load$: load } },
        { provide: ProjectApi, useValue: { load$: loadProjects } }
    ];
    if (options.withScope !== false) {
        providers.push(LinkScope);
    }
    await TestBed.configureTestingModule({
        declarations: [HostComponent],
        imports: [MessageModule, NoopAnimationsModule, TranslateModule.forRoot()],
        providers
    }).compileComponents();
    if (options.withScope !== false) {
        TestBed.inject(LinkScope).idProject.set(options.idProject ?? null);
    }
    const fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    return { fixture, load, loadProjects };
}

function type(fixture: ComponentFixture<HostComponent>, text: string): HTMLDivElement {
    const editor = fixture.nativeElement.querySelector('.editor__content') as HTMLDivElement;
    editor.focus();
    editor.textContent = text;
    const range = document.createRange();
    range.selectNodeContents(editor);
    range.collapse(false);
    const selection = window.getSelection()!;
    selection.removeAllRanges();
    selection.addRange(range);
    editor.dispatchEvent(new Event('input', { bubbles: true }));
    fixture.detectChanges();
    return editor;
}

function options(fixture: ComponentFixture<HostComponent>): string[] {
    return Array.from(
        (fixture.nativeElement as HTMLElement).querySelectorAll('[data-testid="issue-link-option"]')
    ).map(option => option.textContent?.trim() ?? '');
}

function picker(fixture: ComponentFixture<HostComponent>): HTMLElement | null {
    return (fixture.nativeElement as HTMLElement).querySelector(
        '[data-testid="issue-link-picker"]'
    );
}

describe('MessageEditorComponent issue links', () => {
    afterEach(() => TestBed.resetTestingModule());

    it('opens the task picker after #, loads the project issues and filters by the typed number', async () => {
        const { fixture, load, loadProjects } = await render({ idProject: 7 });

        type(fixture, 'see #12');
        expect(load).toHaveBeenCalledWith({
            idProject: 7,
            orderColumn: 'createAt',
            orderDirection: 'desc'
        });
        expect(loadProjects).not.toHaveBeenCalled();
        expect(picker(fixture)).not.toBeNull();
        expect(options(fixture)).toEqual([]);

        load.mock.results[0].value.next(issues);
        fixture.detectChanges();
        expect(options(fixture)).toEqual(['#123Fix login']);

        type(fixture, 'see #45');
        expect(options(fixture)).toEqual(['#456Add dark mode']);
    });

    it('inserts the picked issue as a #reference with a trailing space', async () => {
        const { fixture, load } = await render({ idProject: 7 });
        const editor = type(fixture, 'see #45');
        load.mock.results[0].value.next(issues);
        fixture.detectChanges();

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();

        expect(EditorText.serialize(editor)).toBe('see #456 ');
        expect(picker(fixture)).toBeNull();
    });

    it('refreshes the list in the background when the picker reopens, keeping the old options meanwhile', async () => {
        const { fixture, load } = await render({ idProject: 7 });

        type(fixture, 'see #12');
        load.mock.results[0].value.next(issues);
        fixture.detectChanges();
        expect(options(fixture)).toEqual(['#123Fix login']);

        type(fixture, 'see');
        type(fixture, 'see #12');
        expect(load).toHaveBeenCalledTimes(2);
        expect(options(fixture)).toEqual(['#123Fix login']);

        const refreshed = [issue(123, 'Fix login'), issue(789, 'New task')];
        load.mock.results[1].value.next(refreshed);
        fixture.detectChanges();
        expect(options(fixture)).toEqual(['#123Fix login']);

        type(fixture, 'see #78');
        expect(options(fixture)).toEqual(['#789New task']);
    });

    it('lists the tasks of every visible project when the chat has no project', async () => {
        const { fixture, load, loadProjects } = await render({ idProject: null });

        type(fixture, 'see #1');
        expect(loadProjects).toHaveBeenCalledTimes(1);
        expect(picker(fixture)).not.toBeNull();
        expect(options(fixture)).toEqual([]);

        loadProjects.mock.results[0].value.next(projects);
        expect(load.mock.calls.map(call => call[0].idProject)).toEqual([7, 9]);
        load.mock.results[0].value.next([issue(123, 'Fix login')]);
        load.mock.results[1].value.next([issue(12, 'Blog redesign')]);
        fixture.detectChanges();
        expect(options(fixture)).toEqual(['#123Fix loginRurdesk', '#12Blog redesignWebsite']);
    });

    it('inserts a picked cross-project task as a link carrying its project', async () => {
        const { fixture, load, loadProjects } = await render({ idProject: null });
        const editor = type(fixture, 'see #1');
        loadProjects.mock.results[0].value.next(projects);
        load.mock.results[0].value.next([issue(123, 'Fix login')]);
        load.mock.results[1].value.next([issue(12, 'Blog redesign')]);
        fixture.detectChanges();

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();

        expect(EditorText.serialize(editor)).toBe('see [#123](/project/7/issue/123) ');
        expect(picker(fixture)).toBeNull();
    });

    it('does not open the picker without a link scope', async () => {
        const { fixture, load, loadProjects } = await render({ withScope: false });

        type(fixture, 'see #12');
        expect(load).not.toHaveBeenCalled();
        expect(loadProjects).not.toHaveBeenCalled();
        expect(picker(fixture)).toBeNull();
    });
});

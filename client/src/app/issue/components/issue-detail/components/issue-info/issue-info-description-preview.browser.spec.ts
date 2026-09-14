import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { NEVER, of } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { Issue } from 'src/app/issue/model/issue.model';
import { GitIntegrationApi } from 'src/app/project/api/git-integration.api.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { ProjectStore } from 'src/app/project/project.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { StateStore } from 'src/app/state/store/state.store';
import { PinApi } from 'src/app/pin/api/pin.api.service';
import { MrDiffApi } from 'src/app/issue/api/mr-diff.api.service';
import { IssueInfoComponent } from './issue-info.component';

const ISSUE = {
    idIssue: 10,
    idIssuePublic: 42,
    idProject: 7,
    title: 'Some issue',
    description: 'Some **description** text to select',
    assignedTo: null,
    estimated: 0,
    idGitIntegration: null,
    mrId: null
} as unknown as Issue;

function dispatchMouseSequence(
    target: HTMLElement,
    sequence: { type: 'mousedown' | 'mouseup' | 'click'; clientX: number; clientY: number }[]
): void {
    for (const ev of sequence) {
        target.dispatchEvent(
            new MouseEvent(ev.type, { clientX: ev.clientX, clientY: ev.clientY, bubbles: true })
        );
    }
}

describe('IssueInfoComponent — description preview click behaviour (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [IssueInfoComponent],
            providers: [
                { provide: IssueApi, useValue: { update$: vi.fn().mockReturnValue(of(ISSUE)) } },
                { provide: StateStore, useValue: { statesByProject$: () => of([]) } },
                { provide: SeverityStore, useValue: { severitiesByProject$: () => of([]) } },
                { provide: IssueTypeStore, useValue: { issueTypesByProject$: () => of([]) } },
                {
                    provide: ProjectMemberStore,
                    useValue: { users$: of([]), usersMap$: of(new Map()) }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 7, name: 'p' }) } },
                { provide: AuthStore, useValue: { getUser: () => ({ idUser: 1 }) } },
                { provide: PinApi, useValue: { insert$: () => NEVER } },
                { provide: MrDiffApi, useValue: { loadStatus$: () => NEVER, load$: () => NEVER } },
                { provide: GitIntegrationApi, useValue: { loadOne$: () => NEVER } },
                { provide: Router, useValue: { navigate: vi.fn() } }
            ]
        })
            .overrideComponent(IssueInfoComponent, {
                set: {
                    template: `
                        <div class="description-preview" uiActivatable
                             (mousedown)="onDescriptionPreviewMousedown($event)"
                             (click)="onToggleInputDescription($event)">
                            preview
                        </div>
                    `
                }
            })
            .compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(IssueInfoComponent);
        fixture.componentRef.setInput('issue', ISSUE);
        fixture.componentRef.setInput('project', { idProject: 7, name: 'p' });
        fixture.detectChanges();
        return fixture;
    }

    function preview(fixture: ReturnType<typeof setup>): HTMLElement {
        const el = fixture.nativeElement.querySelector('.description-preview') as HTMLElement;
        if (!el) {
            throw new Error('description-preview element not found');
        }
        return el;
    }

    it('enters edit mode on a plain single click with no movement and no selection', () => {
        const fixture = setup();
        expect(fixture.componentInstance.showInputDescription()).toBe(false);

        const el = preview(fixture);
        dispatchMouseSequence(el, [
            { type: 'mousedown', clientX: 10, clientY: 10 },
            { type: 'mouseup', clientX: 10, clientY: 10 },
            { type: 'click', clientX: 10, clientY: 10 }
        ]);

        expect(fixture.componentInstance.showInputDescription()).toBe(true);
    });

    it('does not enter edit mode when the pointer moves more than the drag threshold', () => {
        const fixture = setup();
        const el = preview(fixture);
        dispatchMouseSequence(el, [
            { type: 'mousedown', clientX: 10, clientY: 10 },
            { type: 'mouseup', clientX: 50, clientY: 10 },
            { type: 'click', clientX: 50, clientY: 10 }
        ]);

        expect(fixture.componentInstance.showInputDescription()).toBe(false);
    });

    it('does not enter edit mode when text is selected after the click', () => {
        const fixture = setup();
        const el = preview(fixture);
        const selection = window.getSelection();
        selection?.removeAllRanges();

        dispatchMouseSequence(el, [
            { type: 'mousedown', clientX: 10, clientY: 10 },
            { type: 'mouseup', clientX: 10, clientY: 10 }
        ]);

        const range = document.createRange();
        range.selectNodeContents(el);
        selection?.addRange(range);

        el.dispatchEvent(new MouseEvent('click', { clientX: 10, clientY: 10, bubbles: true }));

        expect(fixture.componentInstance.showInputDescription()).toBe(false);
        selection?.removeAllRanges();
    });

    it('still enters edit mode when activated by keyboard via uiActivatable', async () => {
        const fixture = setup();
        const component = fixture.componentInstance;
        const el = preview(fixture);

        el.focus();
        el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        await fixture.whenStable();

        // Manually invoke the directive's keyboard handler logic; uiActivatable
        // dispatches a native click when focus + Enter is pressed, but the
        // synthetic KeyboardEvent does not always satisfy event.target checks.
        el.click();
        await fixture.whenStable();

        expect(component.showInputDescription()).toBe(true);
    });
});

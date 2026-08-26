import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { NEVER, of } from 'rxjs';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { AgentRun } from 'src/app/agent/model/agent-run.model';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { GitIntegrationApi } from 'src/app/project/api/git-integration.api.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { ProjectStore } from 'src/app/project/project.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { StateStore } from 'src/app/state/store/state.store';
import { Fixtures } from 'src/testing/fixtures';
import { PinService } from 'src/app/pin/pin.service';
import { MrDiffApi } from 'src/app/issue/api/mr-diff.api.service';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { IssueService } from '../../../../issue.service';
import { Issue } from '../../../../model/issue.model';
import { IssueInfoComponent } from './issue-info.component';

const ISSUE = {
    idIssue: 10,
    idIssuePublic: 42,
    idProject: 7,
    title: 'Some issue',
    description: 'body',
    assignedTo: null,
    estimated: 0
} as unknown as Issue;

const RUN: AgentRun = Fixtures.agentRun();

/**
 * The dock assigns the bot server-side. issue-info autosaves the whole form on
 * every change, so if the control kept the old assignee, the next edit of any
 * field would PATCH it back and un-assign the bot.
 */
describe('IssueInfoComponent — bot assigned from the dock (browser)', () => {
    let issueService: { updateIssue: ReturnType<typeof vi.fn> };

    beforeEach(async () => {
        issueService = { updateIssue: vi.fn().mockReturnValue(of(ISSUE)) };

        await TestBed.configureTestingModule({
            declarations: [IssueInfoComponent],
            providers: [
                { provide: IssueService, useValue: issueService },
                { provide: StateStore, useValue: { statesByProject$: () => of([]) } },
                { provide: SeverityStore, useValue: { severitiesByProject$: () => of([]) } },
                { provide: IssueTypeStore, useValue: { issueTypesByProject$: () => of([]) } },
                {
                    provide: ProjectMemberStore,
                    useValue: { users$: of([]), usersMap$: of(new Map()) }
                },
                {
                    provide: ProjectStore,
                    useValue: { project$: of({ idProject: 7, name: 'p' }) }
                },
                { provide: AuthStore, useValue: { getUser: () => ({ idUser: 1 }) } },
                { provide: I18nService, useValue: { instant: (key: string) => key } },
                { provide: PinService, useValue: { insertPin: () => NEVER } },
                {
                    provide: MrDiffApi,
                    useValue: { getStatus$: () => NEVER, getDiff$: () => NEVER }
                },
                { provide: GitIntegrationApi, useValue: { get$: () => NEVER } },
                { provide: Router, useValue: { navigate: vi.fn() } }
            ]
        })
            .overrideComponent(IssueInfoComponent, { set: { template: '' } })
            .compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(IssueInfoComponent);
        fixture.componentRef.setInput('issue', ISSUE);
        fixture.componentRef.setInput('project', { idProject: 7, name: 'p' });
        fixture.detectChanges();
        return fixture;
    }

    it('syncs the assignee control without saving, so later edits keep the bot', () => {
        const fixture = setup();
        const component = fixture.componentInstance as unknown as {
            onAgentRunCreated: (run: AgentRun) => void;
            assignedToControl: { value: number | null };
            form: { patchValue: (v: Record<string, unknown>) => void };
        };

        component.onAgentRunCreated(RUN);
        fixture.detectChanges();

        expect(component.assignedToControl.value).toBe(8);
        expect(issueService.updateIssue).not.toHaveBeenCalled();

        // A later edit of an unrelated field must carry the bot, not the stale null.
        component.form.patchValue({ title: 'renamed' });
        fixture.detectChanges();

        const saved = issueService.updateIssue.mock.calls.at(-1)?.[0] as Issue | undefined;
        expect(saved?.assignedTo).toBe(8);
    });
});

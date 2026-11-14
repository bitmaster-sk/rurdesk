import { CommonModule } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import en from 'src/assets/i18n/en.json';
import { IssueHistoryField } from 'src/app/issue/constants/issue-history-field.enum';
import {
    IssueHistoryGroup,
    IssueHistoryLookups
} from 'src/app/issue/entity/issue-history-group.entity';
import { IssueHistory, IssueHistoryUser } from 'src/app/issue/model/issue-history.model';
import {
    AvatarStub,
    IssueTypeBadgeStub,
    SeverityCircleStub,
    StateBadgeStub,
    TablerIconStub,
    TagChipStub
} from 'src/testing/stubs';
import { Fixtures } from 'src/testing/fixtures';
import { ActivityHistoryItemComponent } from './activity-history-item.component';

const ALICE: IssueHistoryUser = {
    idUser: 7,
    name: 'Alice',
    colorAvatarBg: '#111',
    avatarInitials: null,
    isActive: true
};
const BOB: IssueHistoryUser = {
    idUser: 8,
    name: 'Bob',
    colorAvatarBg: '#222',
    avatarInitials: null,
    isActive: true
};

const lookups: IssueHistoryLookups = {
    states: new Map([
        [1, Fixtures.state({ idState: 1, name: 'New' })],
        [2, Fixtures.state({ idState: 2, name: 'In progress' })]
    ]),
    severities: new Map([[3, Fixtures.severity({ idSeverity: 3, title: 'High' })]]),
    issueTypes: new Map([[4, Fixtures.issueType({ idIssueType: 4, name: 'Bug' })]]),
    users: new Map([
        [7, ALICE],
        [8, BOB]
    ]),
    tags: new Map([
        [5, Fixtures.tag({ idTag: 5, name: 'next', color: '#00f' })],
        [6, Fixtures.tag({ idTag: 6, name: 'hello', color: '#0f0' })]
    ])
};

function change(field: IssueHistoryField, oldValue: unknown, newValue: unknown): IssueHistory {
    return {
        idIssueHistory: 1,
        idChange: 'a',
        field,
        oldValue,
        newValue,
        idUser: 7,
        source: 'user',
        createAt: '2026-11-10T08:00:00Z'
    };
}

function group(changes: IssueHistory[], source: 'user' | 'system' = 'user'): IssueHistoryGroup {
    return {
        idChange: 'a',
        date: new Date('2026-11-10T08:00:00Z'),
        idUser: source === 'user' ? 7 : null,
        source,
        changes
    };
}

describe('ActivityHistoryItemComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [ActivityHistoryItemComponent],
            imports: [
                CommonModule,
                TranslateModule.forRoot(),
                TablerIconStub,
                AvatarStub,
                StateBadgeStub,
                SeverityCircleStub,
                IssueTypeBadgeStub,
                TagChipStub
            ]
        }).compileComponents();

        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', en);
        translate.use('en');
    });

    function render(input: IssueHistoryGroup, user: IssueHistoryUser | null = ALICE): HTMLElement {
        const fixture = TestBed.createComponent(ActivityHistoryItemComponent);
        fixture.componentRef.setInput('group', input);
        fixture.componentRef.setInput('user', user ?? undefined);
        fixture.componentRef.setInput('lookups', lookups);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function rows(el: HTMLElement): HTMLElement[] {
        return [...el.querySelectorAll<HTMLElement>('[data-testid="history-row"]')];
    }

    function text(el: Element | null): string {
        return (el?.textContent ?? '').replace(/\s+/g, ' ').trim();
    }

    it('shows state and type as badges and severity as a dot with its name', () => {
        const el = render(
            group([
                change(IssueHistoryField.State, 1, 2),
                change(IssueHistoryField.Severity, null, 3),
                change(IssueHistoryField.IssueType, 4, null)
            ])
        );

        const [state, severity, issueType] = rows(el);
        expect([...state.querySelectorAll('app-state-badge')].map(text)).toEqual([
            'New',
            'In progress'
        ]);
        expect(text(severity)).toBe('Severity: No severity → High');
        expect(severity.querySelectorAll('app-severity-circle')).toHaveLength(1);
        expect(severity.querySelector('app-severity-badge')).toBeNull();
        expect(text(issueType)).toBe('Type: Bug → No type');
        expect(issueType.querySelector('app-issue-type-badge')).not.toBeNull();
    });

    it('names a deleted editor instead of leaving the author empty', () => {
        const el = render(group([change(IssueHistoryField.Title, 'Old', 'New')]), null);

        expect(text(el.querySelector('[data-testid="history-deleted-user"]'))).toBe('Deleted user');
        expect(el.querySelector('[data-testid="history-user"]')).toBeNull();
    });

    it('shows the assignee with an avatar and Unassigned for nobody', () => {
        const el = render(group([change(IssueHistoryField.Assignee, null, 8)]));

        const [assignee] = rows(el);
        expect(text(assignee)).toBe('Assignee: Unassigned → Bob');
        expect(assignee.querySelector('[data-testid="history-empty"]')).not.toBeNull();
        expect(assignee.querySelectorAll('app-avatar')).toHaveLength(1);
    });

    it('shows only the added and removed tags as chips', () => {
        const el = render(group([change(IssueHistoryField.Tags, [5], [6])]));

        const [tags] = rows(el);
        const added = tags.querySelector('[data-testid="history-tag-added"]');
        const removed = tags.querySelector('[data-testid="history-tag-removed"]');
        expect(text(added?.querySelector('.tag-sign') ?? null)).toBe('+');
        expect(text(added?.querySelector('app-tag-chip') ?? null)).toBe('hello');
        expect(text(removed?.querySelector('.tag-sign') ?? null)).toBe('−');
        expect(text(removed?.querySelector('app-tag-chip') ?? null)).toBe('next');
        expect(text(tags)).not.toContain('→');
    });

    it('shows No estimate instead of a zero estimate', () => {
        const el = render(group([change(IssueHistoryField.Estimated, 0, 43200)]));

        expect(rows(el).map(text)).toEqual(['Estimate: No estimate → 12h']);
    });

    it('labels a recalculated group as the system and shows no user', () => {
        const el = render(
            group([change(IssueHistoryField.IsSummary, false, true)], 'system'),
            undefined
        );

        expect(text(el.querySelector('[data-testid="history-system"]'))).toBe(
            'Recalculated from subtasks'
        );
        expect(el.querySelector('[data-testid="history-user"]')).toBeNull();
        expect(el.querySelector('app-avatar')).toBeNull();
        expect(rows(el).map(text)).toEqual(['Task kind: regular task → summary task']);
    });

    it('labels a system state change as the system in general', () => {
        const el = render(group([change(IssueHistoryField.State, 1, 2)], 'system'), null);

        expect(text(el.querySelector('[data-testid="history-system"]'))).toBe('System');
    });

    it('shows only that the description changed', () => {
        const el = render(group([change(IssueHistoryField.Description, 'old body', 'new body')]));

        expect(rows(el).map(text)).toEqual(['Changed the description']);
        expect(el.textContent).not.toContain('old body');
    });

    it('shows #id for a state that no longer exists', () => {
        const el = render(group([change(IssueHistoryField.State, 1, 42)]));

        expect(rows(el).map(text)).toEqual(['State: New → #42']);
    });
});

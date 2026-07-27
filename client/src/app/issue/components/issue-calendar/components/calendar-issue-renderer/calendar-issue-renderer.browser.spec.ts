import { CalendarIssueRenderer } from './calendar-issue-renderer';
import { IssueCardViewType } from '../../../../constants/issue-card-view-type.constant';
import { Issue } from '../../../../model/issue.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { User } from 'src/app/auth/model/user.model';

function makeEvt(overrides: {
    issue?: Partial<Issue>;
    severity?: Partial<IssueSeverity>;
    state?: Partial<IssueState>;
    assigned?: Partial<User>;
    isStart?: boolean;
    isEnd?: boolean;
    title?: string;
}): any {
    return {
        isStart: overrides.isStart ?? true,
        isEnd: overrides.isEnd ?? true,
        event: {
            title: overrides.title ?? 'Test Issue',
            extendedProps: {
                issue: {
                    idIssue: 1,
                    idProject: 1,
                    title: overrides.title ?? 'Test Issue',
                    description: '',
                    tracked: 0,
                    estimated: 3600,
                    idState: null,
                    idSeverity: null,
                    ...overrides.issue
                } as Issue,
                severity: overrides.severity
                    ? ({
                          idSeverity: 1,
                          idProject: 1,
                          title: 'High',
                          color: '#ea580c',
                          protected: false,
                          orderRank: 1,
                          ...overrides.severity
                      } as IssueSeverity)
                    : undefined,
                state: overrides.state
                    ? ({
                          idState: 1,
                          idProject: 1,
                          name: 'In Progress',
                          start: false,
                          final: false,
                          protected: false,
                          orderRank: 1,
                          ...overrides.state
                      } as IssueState)
                    : undefined,
                assigned: overrides.assigned
                    ? ({
                          idUser: 1,
                          name: 'John Doe',
                          email: 'j@example.com',
                          colorAvatarBg: '#7c3aed',
                          ...overrides.assigned
                      } as User)
                    : undefined
            }
        }
    };
}

describe('CalendarIssueRenderer', () => {
    let renderer: CalendarIssueRenderer;

    beforeEach(() => {
        renderer = new CalendarIssueRenderer();
    });

    describe('comfortable mode', () => {
        const mode: IssueCardViewType = 'CalendarComfort';

        it('renders a comfortable card with data-issue-id', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({ issue: { idIssue: 42 } }),
                cardMode: mode
            });
            expect(domNodes[0].dataset['issueId']).toBe('42');
            expect(domNodes[0].classList.contains('cal-event-b')).toBe(true);
        });

        it('shows two-line clamped title', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({ title: 'My Title' }),
                cardMode: mode
            });
            const titleEl = domNodes[0].querySelector('.cal-event-b__title');
            expect(titleEl?.textContent).toBe('My Title');
        });

        it('renders state badge when state is provided', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({ state: { name: 'Todo', start: true } }),
                cardMode: mode
            });
            const badge = domNodes[0].querySelector('.cal-badge.cal-state--start');
            expect(badge).toBeTruthy();
            expect(badge?.textContent).toBe('Todo');
        });

        it('renders severity badge when severity is provided', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({ severity: { title: 'Critical', color: '#dc2626' } }),
                cardMode: mode
            });
            const badge = domNodes[0].querySelector('.cal-badge:not([class*="cal-state"])');
            expect(badge?.textContent?.trim()).toBe('Critical');
        });

        it('renders progress bar fill reflecting tracked/estimated ratio', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({ issue: { tracked: 1800, estimated: 3600 } }),
                cardMode: mode
            });
            const fill = domNodes[0].querySelector<HTMLElement>('.cal-event-b__bar-fill');
            expect(fill?.style.width).toBe('50%');
        });

        it('clamps progress bar fill at 100%', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({ issue: { tracked: 7200, estimated: 3600 } }),
                cardMode: mode
            });
            const fill = domNodes[0].querySelector<HTMLElement>('.cal-event-b__bar-fill');
            expect(fill?.style.width).toBe('100%');
        });

        it('renders 0% progress bar when no estimated time', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({ issue: { tracked: 0, estimated: 0 } }),
                cardMode: mode
            });
            const fill = domNodes[0].querySelector<HTMLElement>('.cal-event-b__bar-fill');
            expect(fill?.style.width).toBe('0%');
        });

        it('renders an initials avatar for the assignee', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({
                    assigned: { name: 'Ana Rivera', colorAvatarBg: '#be185d' }
                }),
                cardMode: mode
            });
            const av = domNodes[0].querySelector('.cal-event__avatar');
            expect(av?.textContent?.trim()).toBe('AR');
        });

        it('renders unknown avatar when no assigned user', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({}),
                cardMode: mode
            });
            const av = domNodes[0].querySelector('.cal-event__avatar');
            expect(av?.classList.contains('cal-event__avatar--unknown')).toBe(true);
        });
    });

    describe('compact mode', () => {
        const mode: IssueCardViewType = 'CalendarCompact';

        it('renders a compact card with data-issue-id', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({ issue: { idIssue: 7 } }),
                cardMode: mode
            });
            expect(domNodes[0].dataset['issueId']).toBe('7');
            expect(domNodes[0].classList.contains('cal-event-c')).toBe(true);
        });

        it('shows single-line title', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({ title: 'Compact Title' }),
                cardMode: mode
            });
            const titleEl = domNodes[0].querySelector('.cal-event-c__title');
            expect(titleEl?.textContent).toBe('Compact Title');
        });

        it('shows state badge but no severity badge', () => {
            const { domNodes } = renderer.render({
                evt: makeEvt({ state: { name: 'Done', final: true }, severity: { title: 'Low' } }),
                cardMode: mode
            });
            const badges = domNodes[0].querySelectorAll('.cal-badge');
            expect(badges.length).toBe(1);
            expect(badges[0].classList.contains('cal-state--final')).toBe(true);
        });

        it('has no progress bar', () => {
            const { domNodes } = renderer.render({ evt: makeEvt({}), cardMode: mode });
            expect(domNodes[0].querySelector('.cal-event-b__bar')).toBeNull();
        });
    });
});

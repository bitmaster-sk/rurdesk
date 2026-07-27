import { EventContentArg } from '@fullcalendar/core';
import { User } from 'src/app/auth/model/user.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { Issue } from '../../../../model/issue.model';
import {
    IssueCardViewType,
    isComfortableMode
} from '../../../../constants/issue-card-view-type.constant';

export interface CalendarRenderData {
    evt: EventContentArg;
    cardMode: IssueCardViewType;
}

export class CalendarIssueRenderer {
    render(data: CalendarRenderData): { domNodes: HTMLElement[] } {
        const { evt, cardMode } = data;
        const issue = evt.event.extendedProps['issue'] as Issue;
        const assigned = evt.event.extendedProps['assigned'] as User | undefined;
        const severity = evt.event.extendedProps['severity'] as IssueSeverity | undefined;
        const state = evt.event.extendedProps['state'] as IssueState | undefined;

        const node = isComfortableMode(cardMode)
            ? this.buildComfortable(evt, issue, assigned, severity, state)
            : this.buildCompact(evt, issue, assigned, severity, state);

        return { domNodes: [node] };
    }

    private buildComfortable(
        evt: EventContentArg,
        issue: Issue,
        assigned: User | undefined,
        severity: IssueSeverity | undefined,
        state: IssueState | undefined
    ): HTMLElement {
        const sevColor = severity?.color ?? 'var(--ui-color-unknown)';
        const rS = evt.isStart ? '4px' : '0';
        const rE = evt.isEnd ? '4px' : '0';

        const wrapper = document.createElement('div');
        wrapper.className = 'cal-event-b';
        wrapper.dataset['issueId'] = String(issue.idIssue);
        wrapper.style.setProperty('--sev', sevColor);
        wrapper.style.borderRadius = `${rS} ${rE} ${rE} ${rS}`;

        if (!evt.isStart) wrapper.style.borderLeft = 'none';
        if (!evt.isEnd) wrapper.style.borderRight = 'none';
        this.applyWaveClip(wrapper, evt.isStart, evt.isEnd);

        // Title (up to 2 lines)
        const title = document.createElement('div');
        title.className = 'cal-event-b__title';
        title.innerText = evt.event.title;
        wrapper.appendChild(title);

        // Footer: badges (state + severity) | spacer | name + avatar
        const foot = document.createElement('div');
        foot.className = 'cal-event-b__foot';

        const badges = document.createElement('div');
        badges.className = 'cal-event-b__badges';
        if (state) {
            badges.appendChild(this.buildStateBadge(state));
        }
        if (severity) {
            badges.appendChild(this.buildSeverityBadge(severity));
        }
        foot.appendChild(badges);

        const spacer = document.createElement('div');
        spacer.className = 'cal-event-b__spacer';
        foot.appendChild(spacer);

        const user = document.createElement('div');
        user.className = 'cal-event-b__user';
        const nameEl = document.createElement('span');
        nameEl.className = 'cal-event-b__name';
        nameEl.innerText = assigned ? assigned.name : 'Unassigned';
        user.appendChild(nameEl);
        user.appendChild(this.buildAvatar(assigned, 18));
        foot.appendChild(user);

        wrapper.appendChild(foot);

        // Progress bar
        const pct = this.progressPct(issue);
        const bar = document.createElement('div');
        bar.className = 'cal-event-b__bar';
        const fill = document.createElement('div');
        fill.className = 'cal-event-b__bar-fill';
        fill.style.width = `${pct * 100}%`;
        bar.appendChild(fill);
        wrapper.appendChild(bar);

        return wrapper;
    }

    private buildCompact(
        evt: EventContentArg,
        issue: Issue,
        assigned: User | undefined,
        severity: IssueSeverity | undefined,
        state: IssueState | undefined
    ): HTMLElement {
        const sevColor = severity?.color ?? 'var(--ui-color-unknown)';

        const wrapper = document.createElement('div');
        wrapper.className = 'cal-event-c';
        wrapper.dataset['issueId'] = String(issue.idIssue);
        wrapper.style.setProperty('--sev', sevColor);

        if (!evt.isStart) wrapper.style.borderLeft = 'none';
        if (!evt.isEnd) wrapper.style.borderRight = 'none';
        this.applyWaveClip(wrapper, evt.isStart, evt.isEnd);

        // Title
        const title = document.createElement('span');
        title.className = 'cal-event-c__title';
        title.innerText = issue.title;
        wrapper.appendChild(title);

        // State badge only
        const badges = document.createElement('div');
        badges.className = 'cal-event-c__badges';
        if (state) {
            badges.appendChild(this.buildStateBadge(state));
        }
        wrapper.appendChild(badges);

        // Avatar
        const av = this.buildAvatar(assigned, 16);
        av.classList.add('cal-event-c__av');
        wrapper.appendChild(av);

        return wrapper;
    }

    private buildStateBadge(state: IssueState): HTMLElement {
        const stateClass = state.start ? 'start' : state.final ? 'final' : 'in-progress';
        const badge = document.createElement('span');
        badge.className = `cal-badge cal-state--${stateClass}`;
        badge.innerText = state.name;
        return badge;
    }

    private buildSeverityBadge(severity: IssueSeverity): HTMLElement {
        const badge = document.createElement('span');
        badge.className = 'cal-badge';
        badge.style.setProperty('--c', severity.color);
        badge.innerText = severity.title;
        return badge;
    }

    private buildAvatar(assigned: User | undefined, size: number): HTMLElement {
        const av = document.createElement('div');
        av.className = 'cal-event__avatar';
        av.style.width = `${size}px`;
        av.style.height = `${size}px`;
        av.style.fontSize = `${Math.round(size * 0.38)}px`;

        if (assigned) {
            av.title = assigned.name;
            av.style.backgroundColor = assigned.colorAvatarBg || 'rgba(255,255,255,0.3)';
            av.innerText = this.buildInitials(assigned.name);
        } else {
            av.classList.add('cal-event__avatar--unknown');
            av.title = 'Unassigned';
            av.innerText = '?';
        }

        return av;
    }

    private buildInitials(name: string): string {
        const parts = name.trim().split(' ');
        if (parts.length > 1) {
            return parts
                .slice(0, 2)
                .map(p => p[0])
                .join('')
                .toUpperCase();
        }
        return parts[0].substring(0, 2).toUpperCase();
    }

    /**
     * Applies a clip-path polygon with a zigzag wave on the cut edges of
     * multi-row events. `isStart`/`isEnd` from FullCalendar tell us which
     * edges are cuts (false) vs natural endpoints (true).
     */
    private applyWaveClip(el: HTMLElement, isStart: boolean, isEnd: boolean): void {
        if (isStart && isEnd) return;

        const a = 6; // wave amplitude px
        const n = 12; // half-periods (must be even so last right point hits y=100%)
        const pts: string[] = [];

        // Top edge
        pts.push(isStart ? '0 0' : `${a}px 0`);
        pts.push(isEnd ? '100% 0' : `calc(100% - ${a}px) 0`);

        // Right side top→bottom (wavy when !isEnd)
        if (!isEnd) {
            for (let i = 1; i <= n; i++) {
                const y = ((i / n) * 100).toFixed(1) + '%';
                const x = i % 2 === 1 ? '100%' : `calc(100% - ${a}px)`;
                pts.push(`${x} ${y}`);
            }
        } else {
            pts.push('100% 100%');
        }

        // Bottom-left
        pts.push(isStart ? '0 100%' : `${a}px 100%`);

        // Left side bottom→top (wavy when !isStart)
        if (!isStart) {
            for (let i = n - 1; i >= 1; i--) {
                const y = ((i / n) * 100).toFixed(1) + '%';
                const x = i % 2 === 0 ? `${a}px` : '0';
                pts.push(`${x} ${y}`);
            }
        }

        el.style.clipPath = `polygon(${pts.join(', ')})`;
    }

    private progressPct(issue: Issue): number {
        if (!issue.estimated || issue.estimated <= 0) return 0;
        if (!issue.tracked) return 0;
        return Math.min(issue.tracked / issue.estimated, 1);
    }
}

import { Injectable, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { AclStore } from '../../project/store/acl.store';
import { Command, CommandContext, CommandProvider } from '../../core/command/command.model';

@Injectable({ providedIn: 'root' })
export class TrackerCommandProvider implements CommandProvider {
    private readonly trackerService = inject(TrackerService);
    private readonly acl = inject(AclStore);
    private readonly i18n = inject(I18nService);

    private readonly tracker = toSignal(this.trackerService.tracker$, { initialValue: null });

    public getCommands(ctx: CommandContext): Command[] {
        const commands: Command[] = [];
        const tracker = this.tracker();
        const issue = ctx.issue;

        if (issue && this.acl.canUpdateIssue()) {
            const isThisIssue = tracker?.idIssue === issue.idIssue;
            if (!isThisIssue) {
                commands.push({
                    id: 'tracker.start',
                    title: this.i18n.instant('TRACK.START_HERE'),
                    subtitle: tracker ? this.i18n.instant('TRACK.SWITCH_TITLE') : undefined,
                    group: this.i18n.instant('TRACK.TIME'),
                    icon: 'player-play',
                    keywords: 'timer track time start',
                    modes: ['all', 'commands'],
                    run: () => this.start(issue.idProject, issue.idIssuePublic)
                });
            }
        }

        if (tracker) {
            commands.push(
                {
                    id: 'tracker.toggle-pause',
                    title: this.i18n.instant(tracker.pausedAt ? 'TRACK.RESUME' : 'TRACK.PAUSE'),
                    subtitle: tracker.issueTitle,
                    group: this.i18n.instant('TRACK.TIME'),
                    icon: tracker.pausedAt ? 'player-play' : 'player-pause',
                    keywords: 'timer track time pause resume',
                    modes: ['all', 'commands'],
                    run: () => this.togglePause()
                },
                {
                    id: 'tracker.submit',
                    title: this.i18n.instant('TRACK.SUBMIT'),
                    subtitle: tracker.issueTitle,
                    group: this.i18n.instant('TRACK.TIME'),
                    icon: 'check',
                    keywords: 'timer track time submit confirm',
                    modes: ['all', 'commands'],
                    run: () => this.trackerService.submitTracker$(tracker.idTracker).subscribe()
                },
                {
                    id: 'tracker.discard',
                    title: this.i18n.instant('TRACK.DISCARD'),
                    subtitle: tracker.issueTitle,
                    group: this.i18n.instant('TRACK.TIME'),
                    icon: 'trash',
                    keywords: 'timer track time discard stop',
                    modes: ['all', 'commands'],
                    run: () => this.trackerService.deleteTracker$(tracker.idTracker).subscribe()
                }
            );
        }

        return commands;
    }

    private start(idProject: number, idIssuePublic: number): void {
        const tracker = this.tracker();
        if (tracker) {
            this.trackerService
                .switchTracker$(idProject, idIssuePublic, tracker.idTracker)
                .subscribe();
            return;
        }
        this.trackerService.insertTracker$(idProject, idIssuePublic).subscribe();
    }

    private togglePause(): void {
        const tracker = this.tracker();
        if (!tracker) {
            return;
        }
        const request$ = tracker.pausedAt
            ? this.trackerService.resumeTracker$(tracker.idTracker)
            : this.trackerService.pauseTracker$(tracker.idTracker);
        request$.subscribe();
    }
}

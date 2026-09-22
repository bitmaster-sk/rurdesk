import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    OnDestroy,
    effect,
    inject,
    signal,
    viewChild
} from '@angular/core';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { ActivatedRoute, ParamMap, Router } from '@angular/router';
import { Observable, concat, of } from 'rxjs';
import { filter, map, shareReplay, switchMap } from 'rxjs/operators';
import { ProjectStore } from 'src/app/project/project.store';
import { BrowserTitleService } from 'src/app/core/browser-title.service';
import { toSignal, takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { IssueApi } from '../../api/issue.api.service';
import { IssueConverter } from '../../converter/issue.converter';
import { Issue, CreateIssueReq } from '../../model/issue.model';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { IssueLastViewStorage } from '../../util/issue-last-view.storage';
import { IssueViewMode } from '../../constants/issue-view-modes.enum';
import { IssueDetailPageParams } from './entity/issue-detail-page-params';
import { AgentRunStore } from 'src/app/agent/store/agent-run.store';

@Component({
    selector: 'app-issue-detail',
    templateUrl: './issue-detail.page.html',
    styleUrls: ['./issue-detail.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    providers: [AgentRunStore]
})
export class IssueDetailPage implements OnDestroy {
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly issueApi = inject(IssueApi);
    private readonly projectStore = inject(ProjectStore);
    private readonly notice = inject(NoticeService);
    private readonly savedViewStore = inject(SavedViewStore);
    private readonly toast = inject(ToastNotificationService);
    private readonly browserTitle = inject(BrowserTitleService);
    private readonly i18n = inject(I18nService);
    private readonly commandPalette = inject(CommandPaletteService);
    private readonly deleteConfirmEl = viewChild<ElementRef<HTMLElement>>('deleteConfirm');
    private lastIdProject: number | null = null;

    public readonly agentRunStore = inject(AgentRunStore);
    public readonly project = toSignal(this.projectStore.project$);

    public readonly splitPaneStorageKey = 'rurdesk.issueDetail.split';

    public readonly pendingDeleteIssue = signal<Issue | null>(null);
    public readonly isSplitDialogOpen = signal(false);
    public readonly splitIssue = signal<Issue | null>(null);
    public readonly pendingTrack = signal<Track | null>(null);

    private readonly params$ = this.route.paramMap.pipe(
        map(params => this.toIssueDetailPageParams(params)),
        filter((params): params is IssueDetailPageParams => params !== null)
    );

    // Load the issue once, then patch it live from SubjectIssue notices that
    // target this issue (agent PR link / phase→state mirror / merge poller).
    // The notice carries the full issue, so we swap it in directly — no refetch.
    public readonly issue$ = this.params$.pipe(
        switchMap(params =>
            this.loadIssue(params).pipe(
                switchMap(initial =>
                    concat(
                        of(initial),
                        this.notice.issue$.pipe(
                            filter(n => n.payload?.idIssue === initial.idIssue),
                            map(n => IssueConverter.toIssue(n.payload))
                        )
                    )
                )
            )
        ),
        shareReplay({ bufferSize: 1, refCount: true })
    );

    private readonly issue = toSignal(this.issue$);

    // issue$ re-emits a new object for the SAME issue on every SubjectIssue
    // notice (PR link, phase→state mirror). Track the loaded issue id so the
    // agent run is fetched only when the issue identity actually changes —
    // otherwise redundant refetches race with and clobber the live
    // notice-driven run state, making the panel/timeline flicker or vanish.
    private lastLoadedIdIssue: number | null = null;

    public constructor() {
        effect(() => {
            const issue = this.issue();
            const project = this.project();
            if (!issue?.idIssuePublic || !issue.idIssue || !project?.idProject) {
                return;
            }
            if (issue.idIssue === this.lastLoadedIdIssue) {
                return;
            }
            this.lastLoadedIdIssue = issue.idIssue;
            this.agentRunStore.loadForIssue(project.idProject, issue.idIssuePublic, issue.idIssue);
        });

        this.effectBrowserTabTitle();

        // Feed the open issue into the palette so `>` actions target it contextually.
        this.issue$.pipe(takeUntilDestroyed()).subscribe(issue => {
            this.lastIdProject = issue.idProject ?? null;
            this.commandPalette.setContext({ idProject: issue.idProject ?? null, issue });
        });
    }

    public ngOnDestroy(): void {
        // Stop `>` actions targeting a left issue; the layout nulls idProject if the whole
        // project is being left (children destroy before the layout).
        this.browserTitle.setDefault();
        this.commandPalette.setContext({ idProject: this.lastIdProject, issue: null });
    }

    private effectBrowserTabTitle(): void {
        // Keep the browser tab title in sync with the open issue. The number
        // is first so it survives truncation in narrow tabs; the title is
        // truncated in the helper so long names stay readable in history.
        effect(() => {
            const issue = this.issue();
            if (!issue) {
                return;
            }
            this.browserTitle.setIssueTitle(issue);
        });
    }

    public onTrackAdded(track: Track): void {
        this.pendingTrack.set(track);
    }

    public onSplitRequested(issue: Issue): void {
        this.splitIssue.set(issue);
        this.isSplitDialogOpen.set(true);
    }

    public onSplitAccepted(_children: Issue[]): void {
        this.isSplitDialogOpen.set(false);
    }

    public onSplitCancelled(): void {
        this.isSplitDialogOpen.set(false);
    }

    public onDeleteRequested(issue: Issue): void {
        this.pendingDeleteIssue.set(issue);
        // The uiConfirm directive opens on click; render the host synchronously and click it.
        queueMicrotask(() => this.deleteConfirmEl()?.nativeElement.click());
    }

    public onDeleteConfirmed(): void {
        const issue = this.pendingDeleteIssue();
        this.pendingDeleteIssue.set(null);
        if (!issue) {
            return;
        }
        this.issueApi.delete$(issue.idProject, issue.idIssuePublic).subscribe({
            next: () => {
                this.toast.showSuccess('ISSUE.DELETE.SUCCESS');
                const lastView = IssueLastViewStorage.load(issue.idProject) ?? IssueViewMode.TABLE;
                const idSavedView = this.savedViewStore.idAppliedView();
                const queryParams = idSavedView !== null ? { view: idSavedView } : undefined;
                void this.router.navigate(
                    ['/project', issue.idProject, 'issue', 'view', lastView],
                    {
                        queryParams
                    }
                );
            },
            error: () => {
                this.toast.showError('ISSUE.DELETE.FAILED');
            }
        });
    }

    private loadIssue(params: IssueDetailPageParams): Observable<Issue> {
        if (params.idIssuePublic !== null) {
            return this.issueApi.loadOne$(params.idProject, params.idIssuePublic);
        }
        const cloneSource = (history.state as { cloneSource?: Issue } | null)?.cloneSource;
        // idIssue/idIssuePublic stay absent on purpose — the form reads their absence as "new issue".
        const draft: CreateIssueReq = cloneSource
            ? {
                  idProject: params.idProject,
                  idState: cloneSource.idState,
                  idSeverity: cloneSource.idSeverity,
                  idIssueType: cloneSource.idIssueType,
                  title: this.i18n.instant('ISSUE.COPY_SUFFIX', { title: cloneSource.title }),
                  description: cloneSource.description,
                  assignedTo: cloneSource.assignedTo,
                  estimated: cloneSource.estimated,
                  points: cloneSource.points,
                  scheduledAt: cloneSource.scheduledAt,
                  tracked: 0
              }
            : {
                  idProject: params.idProject,
                  idState: null,
                  idSeverity: null,
                  idIssueType: null,
                  title: '',
                  description: '',
                  tracked: 0
              };
        return of(draft as Issue);
    }

    private toIssueDetailPageParams(params: ParamMap): IssueDetailPageParams | null {
        if (!params.get('idProject') || !params.get('idIssuePublic')) {
            return null;
        }
        const idProject = Number(params.get('idProject'));
        const idIssuePublic = Number(params.get('idIssuePublic'));
        return {
            idProject,
            idIssuePublic: idIssuePublic === 0 ? null : idIssuePublic
        };
    }
}

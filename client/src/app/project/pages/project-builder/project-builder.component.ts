import {
    ChangeDetectionStrategy,
    Component,
    inject,
    OnInit,
    OnDestroy,
    signal,
    computed,
    model
} from '@angular/core';
import { Router } from '@angular/router';
import { HttpErrorResponse } from '@angular/common/http';
import { combineLatest, of, Subscription } from 'rxjs';
import { switchMap, take } from 'rxjs/operators';
import { StagedIssueNode } from '../../model/staged-issue-node.model';
import { ProjectBuilderApi } from '../../api/project-builder.api.service';
import { ProjectStore } from '../../project.store';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { StateStore } from '../../../state/store/state.store';
import { SeverityStore } from '../../../severity/store/severity.store';
import { IssueState } from '../../../state/model/issue-state.model';
import { IssueSeverity } from '../../../severity/model/issue-severity.model';
import { ProjectBuilderIssue, StagingSnapshot } from '../../model/project-builder.model';
import { IssueChangeEvent } from '../../components/project-builder-step-staging/project-builder-step-staging.component';
import { StagedIssuesTree } from './staged-issues-tree';

const STAGING_TTL_MS = 24 * 60 * 60 * 1000; // 24 hours

function stagingKey(idProject: number): string {
    return `pb_staging_${idProject}`;
}

type Step = 0 | 1 | 2 | 3;

@Component({
    selector: 'app-project-builder',
    templateUrl: './project-builder.component.html',
    styleUrls: ['./project-builder.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ProjectBuilderComponent implements OnInit, OnDestroy {
    public step = signal<Step>(0);

    public description = model('');

    public defaultIdState = model<number | null>(null);

    public defaultIdSeverity = model<number | null>(null);

    public stagedIssues = signal<StagedIssueNode[]>([]);

    public summary = signal('');

    public isGenerating = signal(false);

    public isAccepting = signal(false);

    public rateLimitCountdown = signal(0);

    public createdCount = signal(0);

    public createdIdProject = signal<number | null>(null);

    public isRestoreBannerVisible = signal(false);

    public restoredFrom = signal('');

    public states = signal<IssueState[]>([]);

    public severities = signal<IssueSeverity[]>([]);

    private idProject: number | null = null;

    private subscriptions = new Subscription();

    private rateLimitInterval: ReturnType<typeof setInterval> | null = null;

    public readonly minDescriptionChars = 10;

    public isGenerateDisabled = computed(
        () =>
            this.isGenerating() ||
            this.description().length < this.minDescriptionChars ||
            this.rateLimitCountdown() > 0
    );

    public flatStagedIssues = computed(() => StagedIssuesTree.fromTree(this.stagedIssues()));

    private readonly router = inject(Router);

    private readonly projectBuilderApi = inject(ProjectBuilderApi);

    private readonly projectStore = inject(ProjectStore);

    private readonly stateStore = inject(StateStore);

    private readonly severityStore = inject(SeverityStore);

    private readonly toastNotification = inject(ToastNotificationService);

    public ngOnInit(): void {
        this.subscriptions.add(
            this.projectStore.project$
                .pipe(
                    take(1),
                    switchMap(project =>
                        combineLatest([
                            of(project),
                            this.stateStore.statesByProject$(project.idProject),
                            this.severityStore.severitiesByProject$(project.idProject)
                        ])
                    )
                )
                .subscribe(([project, states, severities]) => {
                    if (!this.idProject) {
                        this.idProject = project.idProject!;
                        this.defaultIdState.set(project.idStateDefault ?? null);
                        this.defaultIdSeverity.set(project.idSeverityDefault ?? null);
                        this.tryRestoreStaging();
                    }
                    this.states.set(states);
                    this.severities.set(severities);
                })
        );
    }

    public ngOnDestroy(): void {
        this.subscriptions.unsubscribe();
        this.clearRateLimitInterval();
    }

    // Step 1 actions

    public onGenerate(): void {
        if (!this.idProject || this.isGenerateDisabled()) {
            return;
        }
        this.isGenerating.set(true);

        this.projectBuilderApi
            .generate$(this.idProject, {
                description: this.description(),
                idState: this.defaultIdState(),
                idSeverity: this.defaultIdSeverity()
            })
            .subscribe({
                next: res => {
                    this.summary.set(res.summary);
                    this.stagedIssues.set(StagedIssuesTree.toTree(res.issues));
                    this.isGenerating.set(false);
                    this.step.set(2);
                    this.saveStaging();
                },
                error: (err: HttpErrorResponse) => {
                    this.isGenerating.set(false);
                    if (err.status === 429) {
                        const retryAfter = Number(err.headers.get('Retry-After') ?? 30);
                        this.startRateLimitCountdown(retryAfter);
                    } else {
                        this.toastNotification.showError('PROJECT_BUILDER.ERROR.GENERATE_FAILED');
                    }
                }
            });
    }

    // Step 2

    public onAccept(): void {
        if (!this.idProject) {
            return;
        }
        this.isAccepting.set(true);

        this.projectBuilderApi.accept$(this.idProject, this.flatStagedIssues()).subscribe({
            next: res => {
                this.isAccepting.set(false);
                this.createdCount.set(res.issues.length);
                this.createdIdProject.set(this.idProject);
                this.clearStaging();
                this.step.set(3);
            },
            error: (err: HttpErrorResponse) => {
                this.isAccepting.set(false);
                if (err.status === 422) {
                    this.toastNotification.showError('PROJECT_BUILDER.ERROR.STAGING_INVALID');
                } else {
                    this.toastNotification.showError('PROJECT_BUILDER.ERROR.SAVE_FAILED');
                }
            }
        });
    }

    public onBack(): void {
        this.step.set(1);
    }

    // Confirmed by uiConfirm on the button; reaching here means the user agreed.
    public onRegenerate(): void {
        this.clearStaging();
        this.stagedIssues.set([]);
        this.summary.set('');
        this.step.set(1);
    }

    public onDeleteNode(node: StagedIssueNode): void {
        this.stagedIssues.update(roots => StagedIssuesTree.removeNode(roots, node));
        this.saveStaging();
    }

    public onStagingIssueChange(event: IssueChangeEvent): void {
        this.onIssueChange(event.node, event.updated);
    }

    public onIssueChange(node: StagedIssueNode, updated: ProjectBuilderIssue): void {
        Object.assign(node.data, updated);
        this.stagedIssues.update(roots => [...roots]);
        this.saveStaging();
    }

    public onDismissBanner(): void {
        this.isRestoreBannerVisible.set(false);
    }

    public onDiscardStaging(): void {
        this.clearStaging();
        this.stagedIssues.set([]);
        this.summary.set('');
        this.step.set(1);
        this.isRestoreBannerVisible.set(false);
    }

    // Step 3

    public onGoToIssues(): void {
        if (this.createdIdProject()) {
            void this.router.navigate([
                '/project',
                this.createdIdProject(),
                'issue',
                'view',
                'table'
            ]);
        }
    }

    // Staging persistence
    private saveStaging(): void {
        if (!this.idProject) {
            return;
        }
        const snapshot: StagingSnapshot = {
            summary: this.summary(),
            issues: this.flatStagedIssues(),
            generatedAt: new Date().toISOString()
        };
        localStorage.setItem(stagingKey(this.idProject), JSON.stringify(snapshot));
    }

    private clearStaging(): void {
        if (this.idProject) {
            localStorage.removeItem(stagingKey(this.idProject));
        }
    }

    private tryRestoreStaging(): void {
        if (!this.idProject) {
            this.step.set(1);
            return;
        }
        const raw = localStorage.getItem(stagingKey(this.idProject));
        if (!raw) {
            this.step.set(1);
            return;
        }
        try {
            const parsed: unknown = JSON.parse(raw);
            if (!ProjectBuilderComponent.isStagingSnapshot(parsed)) {
                this.clearStaging();
                this.step.set(1);
                return;
            }
            const snapshot: StagingSnapshot = parsed;
            const age = Date.now() - new Date(snapshot.generatedAt).getTime();
            if (age > STAGING_TTL_MS) {
                this.clearStaging();
                this.step.set(1);
                return;
            }
            this.summary.set(snapshot.summary);
            this.stagedIssues.set(StagedIssuesTree.toTree(snapshot.issues));
            this.restoredFrom.set(new Date(snapshot.generatedAt).toLocaleString());
            this.isRestoreBannerVisible.set(true);
            this.step.set(2);
        } catch {
            this.clearStaging();
            this.step.set(1);
        }
    }

    private static isStagingSnapshot(value: unknown): value is StagingSnapshot {
        if (value === null || typeof value !== 'object') {
            return false;
        }
        const snapshot = value as Record<string, unknown>;
        return (
            typeof snapshot['summary'] === 'string' &&
            typeof snapshot['generatedAt'] === 'string' &&
            Array.isArray(snapshot['issues'])
        );
    }

    // Rate limit handling
    private startRateLimitCountdown(seconds: number): void {
        this.rateLimitCountdown.set(seconds);
        this.clearRateLimitInterval();
        this.rateLimitInterval = setInterval(() => {
            this.rateLimitCountdown.update(v => {
                if (v <= 1) {
                    this.clearRateLimitInterval();
                    return 0;
                }
                return v - 1;
            });
        }, 1000);
    }

    private clearRateLimitInterval(): void {
        if (this.rateLimitInterval !== null) {
            clearInterval(this.rateLimitInterval);
            this.rateLimitInterval = null;
        }
    }
}

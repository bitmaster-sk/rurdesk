import {
    ChangeDetectionStrategy,
    Component,
    ViewEncapsulation,
    computed,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { toSignal } from '@angular/core/rxjs-interop';
import { switchMap } from 'rxjs/operators';
import { Issue } from '../../model/issue.model';
import { ProposedIssue } from '../../model/split.model';
import { SplitApi } from '../../api/split.api.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { StateStore } from 'src/app/state/store/state.store';
import { ProjectStore } from 'src/app/project/project.store';

type Step = 'input' | 'loading' | 'review' | 'saving' | 'done';

@Component({
    selector: 'app-split-dialog',
    templateUrl: './split-dialog.component.html',
    styleUrls: ['./split-dialog.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    // None: the dialog's content is projected into a body-level CDK overlay
    // (ui-dialog), so :host/emulated scoping can't reach it. Selectors below are
    // globally scoped under the `.split-dialog` panelClass instead.
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class SplitDialogComponent {
    // Use inject() so field initializers below can safely reference these
    private splitApi = inject(SplitApi);
    private toast = inject(ToastNotificationService);
    private severityStore = inject(SeverityStore);
    private stateStore = inject(StateStore);
    private projectStore = inject(ProjectStore);

    public idProject = input.required<number>();
    public issue = input.required<Issue>();

    public accepted = output<Issue[]>();
    public cancelled = output<void>();

    public readonly step = signal<Step>('input');
    public readonly children = signal<ProposedIssue[]>([]);
    public readonly acceptedCount = signal(0);

    /** Dialog header title + params, varying by step. */
    private readonly isReview = computed(
        () => this.step() === 'review' || this.step() === 'saving'
    );
    protected readonly headerKey = computed(() =>
        this.isReview() ? 'SPLIT.REVIEW_TITLE' : 'SPLIT.TITLE'
    );
    protected readonly headerParams = computed(() =>
        this.isReview() ? { count: this.children().length } : {}
    );
    /** Block close while an async operation is in flight. */
    protected readonly isBusy = computed(
        () => this.step() === 'loading' || this.step() === 'saving'
    );

    private readonly project$ = this.projectStore.project$;

    public readonly severities = toSignal(
        this.project$.pipe(
            switchMap(project => this.severityStore.severitiesByProject$(project.idProject))
        ),
        { initialValue: [] }
    );

    public readonly states = toSignal(
        this.project$.pipe(
            switchMap(project => this.stateStore.statesByProject$(project.idProject))
        ),
        { initialValue: [] }
    );

    public onSplit(hint: string): void {
        const idProject = this.idProject();
        const idIssuePublic = this.issue().idIssuePublic;
        if (idIssuePublic == null) {
            return;
        }

        this.step.set('loading');

        this.splitApi.preview$(idProject, idIssuePublic, hint).subscribe({
            next: res => {
                const issue = this.issue();
                const startState = this.states().find(s => s.start);
                this.children.set(
                    res.children.map(child => ({
                        ...child,
                        idSeverity: issue.idSeverity,
                        idState: startState?.idState ?? issue.idState
                    }))
                );
                this.step.set('review');
            },
            error: (err: HttpErrorResponse) => {
                this.step.set('input');
                if (err.status === 429) {
                    this.toast.showError('SPLIT.ERROR_RATE_LIMIT');
                } else {
                    this.toast.showError('SPLIT.ERROR_AI');
                }
            }
        });
    }

    public onAccept(children: ProposedIssue[]): void {
        const idProject = this.idProject();
        const idIssuePublic = this.issue().idIssuePublic;

        if (idIssuePublic == null || children.length === 0) {
            return;
        }

        this.step.set('saving');

        this.splitApi.accept$(idProject, idIssuePublic, children).subscribe({
            next: res => {
                this.acceptedCount.set(res.children.length);
                this.step.set('done');
                this.accepted.emit(res.children);
            },
            error: () => {
                this.step.set('review');
                this.toast.showError('SPLIT.ERROR_AI');
            }
        });
    }

    public onCancel(): void {
        this.cancelled.emit();
    }
}

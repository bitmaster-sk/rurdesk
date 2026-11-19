import {
    ChangeDetectionStrategy,
    Component,
    OnInit,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormBuilder, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { finalize } from 'rxjs/operators';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { Project } from 'src/app/project/model/project.model';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { StateStore } from 'src/app/state/store/state.store';
import { TASK_DRAFT_TITLE_MAX, TaskDraftConverter } from '../../entity/task-draft.converter';

/**
 * The owner edits the agent's task draft and saves it as an ordinary issue of the
 * picked project, then lands on it. Saving starts no agent run.
 */
@Component({
    selector: 'app-task-draft-dialog',
    templateUrl: './task-draft-dialog.component.html',
    styleUrls: ['./task-draft-dialog.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class TaskDraftDialogComponent implements OnInit {
    private readonly fb = inject(FormBuilder);
    private readonly issueApi = inject(IssueApi);
    private readonly router = inject(Router);
    private readonly i18n = inject(I18nService);
    private readonly states = toSignal(inject(StateStore).states$, { initialValue: [] });

    /** The draft message: Markdown whose first `# ` line is the title. */
    public readonly body = input.required<string>();
    public readonly idConversation = input.required<number>();
    /** The conversation's main project, preselected. */
    public readonly idProject = input<number | null>(null);
    /** The projects the task can be saved in. */
    public readonly projects = input<Project[]>([]);

    public readonly cancelled = output<void>();

    protected readonly titleMax = TASK_DRAFT_TITLE_MAX;
    protected readonly isSaving = signal(false);

    protected readonly form = this.fb.group({
        title: this.fb.nonNullable.control('', [
            Validators.required,
            Validators.maxLength(TASK_DRAFT_TITLE_MAX),
            Validators.pattern(/\S/)
        ]),
        description: this.fb.nonNullable.control(''),
        idProject: this.fb.control<number | null>(null, Validators.required)
    });

    public ngOnInit(): void {
        const draft = TaskDraftConverter.toDraft(this.body());
        this.form.reset({ ...draft, idProject: this.idProject() });
    }

    protected onSave(): void {
        const { title, description, idProject } = this.form.getRawValue();
        if (this.form.invalid || idProject === null || this.isSaving()) {
            return;
        }
        this.isSaving.set(true);
        const project = this.projects().find(p => p.idProject === idProject);
        const linkLine = this.i18n.instant('AGENT_CHAT.TASK_DRAFT.LINK_LINE', {
            url: `/agent-chat/${this.idConversation()}`
        });
        this.issueApi
            .insert$({
                idProject,
                title: title.trim(),
                description: TaskDraftConverter.toDescription(description, linkLine),
                idState: project?.idStateDefault ?? this.startState(idProject),
                idSeverity: project?.idSeverityDefault ?? null,
                idIssueType: project?.idIssueTypeDefault ?? null,
                tracked: 0
            })
            .pipe(finalize(() => this.isSaving.set(false)))
            .subscribe(created => {
                void this.router.navigate(['/project', idProject, 'issue', created.idIssuePublic]);
            });
    }

    // Without a project default: the start state, else the first one, as the quick create picks.
    private startState(idProject: number): number | null {
        const states = this.states().filter(state => state.idProject === idProject);
        const state =
            states.find(s => s.start) ?? [...states].sort((a, b) => a.orderRank - b.orderRank)[0];
        return state?.idState ?? null;
    }
}

import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    inject,
    input,
    output,
    signal,
    untracked
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, Validators } from '@angular/forms';
import {
    catchError,
    concatMap,
    distinctUntilChanged,
    from,
    map,
    of,
    switchMap,
    toArray
} from 'rxjs';
import { ApiError } from 'src/app/shared/model/api-error.model';
import { Project } from 'src/app/project/model/project.model';
import { AgentConversationApi } from '../../api/agent-conversation.api.service';
import { ProjectAttachOutcome } from '../../constants/project-attach-outcome.enum';
import {
    AgentConversation,
    AgentConversationAgent,
    AgentConversationSettings
} from '../../model/agent-conversation.model';

const NOT_IN_PROJECT_KEY = 'error.agent_not_in_project';

interface AttachResult {
    idProject: number;
    outcome: ProjectAttachOutcome;
}

/**
 * Creates a conversation with its projects, agent and model.
 * The first project is the main one; the others are attached after the conversation exists.
 */
@Component({
    selector: 'app-new-conversation',
    templateUrl: './new-conversation.component.html',
    styleUrls: ['./new-conversation.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class NewConversationComponent {
    public readonly projects = input<Project[]>([]);
    /** Preselected main project. */
    public readonly idProject = input<number | null>(null);

    public readonly created = output<AgentConversation>();
    /** Leaves the form for the list; only offered where the two are not side by side. */
    public readonly back = output<void>();
    /** The projects the server refused to attach: the agent or the owner is not a member. */
    public readonly attachRefused = output<Project[]>();
    /** The projects whose attach failed for any other reason. */
    public readonly attachFailed = output<Project[]>();

    private readonly api = inject(AgentConversationApi);

    protected readonly form = new FormGroup({
        idProjects: new FormControl<number[]>([], { nonNullable: true }),
        idUserAgent: new FormControl<number | null>(null, Validators.required)
    });
    protected readonly agents = signal<AgentConversationAgent[]>([]);
    protected readonly isCreating = signal(false);
    protected readonly settings = signal<AgentConversationSettings>({ model: null });
    private readonly idUserAgent = signal<number | null>(null);
    protected readonly agentModels = computed(
        () => this.agents().find(agent => agent.idUser === this.idUserAgent()) ?? null
    );
    private readonly idProjects = signal<number[]>([]);
    protected readonly chosen = computed(() => {
        const projects = this.projects();
        return this.idProjects()
            .map(id => projects.find(project => project.idProject === id))
            .filter((project): project is Project => project !== undefined);
    });

    public constructor() {
        effect(() => {
            const idProject = this.idProject();
            untracked(() =>
                this.form.controls.idProjects.setValue(idProject === null ? [] : [idProject])
            );
        });
        this.form.controls.idProjects.valueChanges
            .pipe(takeUntilDestroyed())
            .subscribe(ids => this.idProjects.set(ids));
        this.form.controls.idUserAgent.valueChanges
            .pipe(distinctUntilChanged(), takeUntilDestroyed())
            .subscribe(idUserAgent => {
                this.idUserAgent.set(idUserAgent);
                this.settings.set({ model: null });
            });
        // The agents offered depend on the main project: its members, or without
        // one every agent sharing a project with the owner.
        this.form.controls.idProjects.valueChanges
            .pipe(
                map(ids => ids[0] ?? null),
                distinctUntilChanged(),
                switchMap(idProject => this.api.loadAgents$(idProject)),
                takeUntilDestroyed()
            )
            .subscribe(agents => {
                this.agents.set(agents);
                const idUserAgent = this.form.controls.idUserAgent.value;
                if (!agents.some(agent => agent.idUser === idUserAgent)) {
                    this.form.controls.idUserAgent.setValue(agents[0]?.idUser ?? null);
                }
            });
    }

    protected onMakeMain(idProject: number): void {
        const ids = this.form.controls.idProjects.value;
        this.form.controls.idProjects.setValue([idProject, ...ids.filter(id => id !== idProject)]);
    }

    protected onStart(): void {
        const { idProjects, idUserAgent } = this.form.getRawValue();
        if (idUserAgent === null || this.isCreating()) {
            return;
        }
        const [idMain = null, ...idOthers] = idProjects;
        this.isCreating.set(true);
        this.api
            .insert$({ idUserAgent, idProject: idMain, ...this.settings() })
            .pipe(
                // One failed project does not stop the rest or undo the conversation.
                switchMap(conversation =>
                    from(idOthers).pipe(
                        concatMap(idProject =>
                            this.api.insertProject$(conversation.idConversation, idProject).pipe(
                                map((): AttachResult => ({
                                    idProject,
                                    outcome: ProjectAttachOutcome.Attached
                                })),
                                catchError((err: unknown) =>
                                    of<AttachResult>({
                                        idProject,
                                        outcome:
                                            ApiError.translateKeyOf(err) === NOT_IN_PROJECT_KEY
                                                ? ProjectAttachOutcome.Refused
                                                : ProjectAttachOutcome.Failed
                                    })
                                )
                            )
                        ),
                        toArray(),
                        map(results => ({ conversation, results }))
                    )
                )
            )
            .subscribe({
                next: ({ conversation, results }) => {
                    this.isCreating.set(false);
                    this.created.emit(conversation);
                    const refused = this.projectsOf(results, ProjectAttachOutcome.Refused);
                    if (refused.length > 0) {
                        this.attachRefused.emit(refused);
                    }
                    const failed = this.projectsOf(results, ProjectAttachOutcome.Failed);
                    if (failed.length > 0) {
                        this.attachFailed.emit(failed);
                    }
                },
                error: () => this.isCreating.set(false)
            });
    }

    private projectsOf(results: AttachResult[], outcome: ProjectAttachOutcome): Project[] {
        const projects = this.projects();
        return results
            .filter(result => result.outcome === outcome)
            .map(result => projects.find(project => project.idProject === result.idProject))
            .filter((project): project is Project => project !== undefined);
    }
}

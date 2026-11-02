import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    computed,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { FormControl } from '@angular/forms';
import { forkJoin } from 'rxjs';
import { AgentRunApi } from 'src/app/agent/api/agent-run.api.service';
import { AgentStagePlanEditorRow } from 'src/app/agent/entity/agent-stage-plan-editor.entity';
import { AgentOverview } from 'src/app/agent/model/agent-overview.model';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { User } from 'src/app/auth/model/user.model';
import { ProjectSkillApi } from 'src/app/project/api/project-skill.api.service';
import { SkillApi } from 'src/app/shared/api/skill.api.service';
import { Skill } from 'src/app/shared/model/skill.model';
import { StartReleaseAgentReq } from '../../model/release.model';

@Component({
    selector: 'app-release-agent-form',
    templateUrl: './release-agent-form.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReleaseAgentFormComponent implements OnInit {
    public readonly idProject = input.required<number>();
    public readonly agents = input<User[]>([]);
    public readonly hasNotes = input(false);
    public readonly isBusy = input(false);

    public readonly started = output<StartReleaseAgentReq>();
    public readonly cancelled = output<void>();

    private readonly skillApi = inject(SkillApi);
    private readonly projectSkillApi = inject(ProjectSkillApi);
    private readonly agentRunApi = inject(AgentRunApi);
    private readonly destroyRef = inject(DestroyRef);

    protected readonly idUserAgent = new FormControl<number | null>(null);
    private readonly idUserAgentValue = toSignal(this.idUserAgent.valueChanges, {
        initialValue: null
    });

    protected readonly skills = signal<Skill[]>([]);
    private readonly overviews = signal<AgentOverview[]>([]);
    private readonly idsSkill = signal<number[]>([]);
    private readonly stageModel = signal<string | null>(null);
    private readonly stageTurns = signal<number | null>(null);

    protected readonly overview = computed(
        () => this.overviews().find(item => item.idUserAgent === this.idUserAgentValue()) ?? null
    );
    protected readonly models = computed(() => this.overview()?.models ?? []);
    protected readonly row = computed<AgentStagePlanEditorRow>(() => ({
        stage: AgentStage.ReleaseNotes,
        idsSkill: this.idsSkill(),
        model: this.stageModel(),
        maxTurns: this.stageTurns(),
        defaultMaxTurns: this.overview()?.maxTurnsPlan ?? null,
        lock: null
    }));

    public ngOnInit(): void {
        this.idUserAgent.setValue(this.agents()[0]?.idUser ?? null);
        this.idUserAgent.valueChanges
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.stageModel.set(this.overview()?.modelDefault ?? null));
        forkJoin({
            skills: this.skillApi.load$(),
            defaults: this.projectSkillApi.load$(this.idProject()),
            overviews: this.agentRunApi.loadAgentsOverview$(this.idProject())
        })
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(({ skills, defaults, overviews }) => {
                this.skills.set(skills);
                this.overviews.set(overviews);
                this.stageModel.set(this.overview()?.modelDefault ?? null);
                this.idsSkill.set(
                    defaults
                        .filter(entry => entry.stage === AgentStage.ReleaseNotes)
                        .map(entry => entry.idSkill)
                );
            });
    }

    protected onSkillToggled(idSkill: number): void {
        const ids = this.idsSkill();
        this.idsSkill.set(
            ids.includes(idSkill) ? ids.filter(id => id !== idSkill) : [...ids, idSkill]
        );
    }

    protected onModelPicked(model: string | null): void {
        this.stageModel.set(model);
    }

    protected onMaxTurns(maxTurns: number | null): void {
        this.stageTurns.set(maxTurns);
    }

    protected onStart(): void {
        const idUserAgent = this.idUserAgent.value;
        if (idUserAgent === null) {
            return;
        }
        const stage = AgentStage.ReleaseNotes;
        const model = this.stageModel();
        const maxTurns = this.stageTurns();
        this.started.emit({
            idUserAgent,
            idsSkillByStage: { [stage]: this.idsSkill() },
            isShortWorkflow: false,
            ...(model ? { modelByStage: { [stage]: model } } : {}),
            ...(maxTurns ? { maxTurnsByStage: { [stage]: maxTurns } } : {})
        });
    }

    protected onCancel(): void {
        this.cancelled.emit();
    }
}

import {
    Component,
    DestroyRef,
    computed,
    forwardRef,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { User } from 'src/app/auth/model/user.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { AgentRunApi } from '../../agent/api/agent-run.api.service';
import { AgentRun } from '../../agent/model/agent-run.model';
import { UiSelectComponent } from '../../ui/components/select/select.component';
import { AgentOverview } from '../../agent/model/agent-overview.model';

// `optionValue="idUser"`, so the bound value is an id (multi: a list of ids).
type UserDropdownValue = number | number[] | null;

@Component({
    selector: 'app-user-dropdown',
    templateUrl: './user-dropdown.component.html',
    styleUrls: ['./user-dropdown.component.scss'],
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => UserDropdownComponent),
            multi: true
        }
    ],
    standalone: false
})
export class UserDropdownComponent implements ControlValueAccessor {
    public readonly multi = input(false);
    public readonly filter = input(false);
    public readonly appendTo = input<string | null>(null);
    public readonly users = input<User[]>([]);
    public readonly saveStatus = input<UiSaveState>(UiSaveState.Idle);

    public readonly hasAgentFeatures = input(false);
    public readonly idProject = input<number | null>(null);
    public readonly idIssuePublic = input<number | null>(null);
    public readonly agentRunCreated = output<AgentRun>();

    private readonly agentRunApi = inject(AgentRunApi);
    private readonly destroyRef = inject(DestroyRef);
    private readonly select = viewChild(UiSelectComponent);

    protected readonly overviewByAgent = signal<Map<number, AgentOverview>>(new Map());

    protected readonly sortedUsers = computed(() => {
        const users = this.users();
        if (!this.hasAgentFeatures()) {
            return users;
        }
        return [...users].sort((left, right) => {
            if (!!left.isBot !== !!right.isBot) {
                return left.isBot ? 1 : -1;
            }
            return left.name.localeCompare(right.name);
        });
    });

    public value: UserDropdownValue = null;

    public set selected(value: UserDropdownValue) {
        this.value = value;
        this.onChange(value);
        this.onTouch(value);
    }

    public get selected(): UserDropdownValue {
        return this.value;
    }

    public onChange: (value: UserDropdownValue) => void = () => {};
    public onTouch: (value: UserDropdownValue) => void = () => {};

    public writeValue(value: UserDropdownValue): void {
        this.value = value;
    }

    public registerOnChange(fn: (value: UserDropdownValue) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: (value: UserDropdownValue) => void): void {
        this.onTouch = fn;
    }

    protected overviewOf(idUser: number): AgentOverview | null {
        return this.overviewByAgent().get(idUser) ?? null;
    }

    protected onOpened(): void {
        const idProject = this.idProject();
        if (!this.hasAgentFeatures() || idProject === null) {
            return;
        }
        this.agentRunApi
            .agentsOverview$(idProject)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: all =>
                    this.overviewByAgent.set(new Map(all.map(row => [row.idUserBot, row]))),
                error: () => this.overviewByAgent.set(new Map())
            });
    }

    // Writes the value WITHOUT emitting: the run already exists, and onChange would
    // re-enter the assignee-change path and create a second one.
    protected onAgentRunCreated(run: AgentRun): void {
        this.value = run.idUserBot;
        this.agentRunCreated.emit(run);
        this.select()?.closePanel();
    }
}

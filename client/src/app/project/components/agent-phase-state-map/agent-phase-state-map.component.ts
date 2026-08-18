import {
    ChangeDetectionStrategy,
    Component,
    OnDestroy,
    OnInit,
    inject,
    input,
    signal
} from '@angular/core';
import { FormArray, FormBuilder, FormControl } from '@angular/forms';
import { Subscription } from 'rxjs';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { PhaseStateMapApi } from '../../api/phase-state-map.api.service';
import {
    AGENT_PHASES,
    AGENT_PHASE_LABELS,
    PhaseStateMappingEntry
} from '../../model/phase-state-mapping.model';
import { StateStore } from '../../../state/store/state.store';
import { IssueState } from '../../../state/model/issue-state.model';
import { Project } from '../../model/project.model';

@Component({
    selector: 'app-agent-phase-state-map',
    templateUrl: './agent-phase-state-map.component.html',
    styleUrls: ['./agent-phase-state-map.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentPhaseStateMapComponent implements OnInit, OnDestroy {
    public readonly project = input.required<Project>();

    private readonly phaseStateMapApi = inject(PhaseStateMapApi);
    private readonly stateStore = inject(StateStore);
    private readonly formBuilder = inject(FormBuilder);
    private readonly toast = inject(ToastNotificationService);

    public readonly phases = AGENT_PHASES;
    public readonly phaseLabels = AGENT_PHASE_LABELS;
    public readonly isLoading = signal(true);

    /** Per-row auto-save status, keyed by row index. */
    private readonly rowStatus = signal<Record<number, UiSaveState>>({});

    public readonly states = signal<IssueState[]>([]);
    public mappingsFormArray!: FormArray<FormControl<number | null>>;

    private readonly subscription = new Subscription();

    public ngOnInit(): void {
        const idProject = this.project().idProject;

        // The two sources are deliberately NOT combined. The rows are fixed
        // (AGENT_PHASES), so the form is built once from the saved mappings, while
        // states stay live because they are edited in a sibling panel on this same
        // settings page. Rebuilding the form on every states emission — as a
        // combineLatest does — discards a pick the user has not saved yet and
        // re-registers a valueChanges subscription per row on every edit.
        this.loadMappings();

        this.subscription.add(
            this.stateStore.statesByProject$(idProject).subscribe(states => {
                this.states.set(states);
                if (this.hasStaleMapping(states)) {
                    this.loadMappings();
                }
            })
        );
    }

    private loadMappings(): void {
        this.isLoading.set(true);
        this.subscription.add(
            this.phaseStateMapApi.load$(this.project().idProject).subscribe({
                next: mappings => this.buildForm(mappings),
                error: () => {
                    this.isLoading.set(false);
                    this.toast.showError('AGENT.PHASE_STATE_MAP.LOAD_ERROR');
                }
            })
        );
    }

    private hasStaleMapping(states: IssueState[]): boolean {
        if (!this.mappingsFormArray) {
            return false;
        }
        const idsState = new Set(states.map(state => state.idState));
        return this.mappingsFormArray.controls.some(
            control => control.value !== null && !idsState.has(control.value)
        );
    }

    private buildForm(mappings: PhaseStateMappingEntry[]): void {
        const mappingByPhase = new Map(mappings.map(m => [m.phase, m.idState]));
        this.mappingsFormArray = this.formBuilder.array(
            AGENT_PHASES.map(phase =>
                this.formBuilder.control<number | null>(
                    (mappingByPhase.get(phase) ?? null) as number | null
                )
            )
        );
        // Each row auto-saves on change (pick = commit). Construction doesn't emit,
        // so only user edits trigger a save.
        this.mappingsFormArray.controls.forEach((control, index) => {
            this.subscription.add(control.valueChanges.subscribe(() => this.onSaveRow(index)));
        });
        this.isLoading.set(false);
    }

    public ngOnDestroy(): void {
        this.subscription.unsubscribe();
    }

    public getMappingControl(index: number): FormControl<number | null> {
        return this.mappingsFormArray.at(index);
    }

    public rowSaveStatus(index: number): UiSaveState {
        return this.rowStatus()[index] ?? UiSaveState.Idle;
    }

    private onSaveRow(index: number): void {
        this.setRowStatus(index, UiSaveState.Saving);
        const idProject = this.project().idProject;
        const mappings: PhaseStateMappingEntry[] = AGENT_PHASES.map((phase, i) => ({
            phase: phase,
            idState: this.getMappingControl(i).value
        })).filter(entry => entry.idState !== null);

        this.phaseStateMapApi.replace$(idProject, mappings).subscribe({
            next: () => this.setRowStatus(index, UiSaveState.Saved),
            error: () => {
                this.setRowStatus(index, UiSaveState.Error);
                this.toast.showError('AGENT.PHASE_STATE_MAP.SAVE_ERROR');
            }
        });
    }

    private setRowStatus(index: number, status: UiSaveState): void {
        this.rowStatus.update(m => ({ ...m, [index]: status }));
    }
}

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
import { WorkflowEventMapApi } from '../../api/workflow-event-map.api.service';
import { WorkflowEventMappingEntry } from '../../model/workflow-event-mapping.model';
import { AgentPhase, PHASE_LABELS } from '../../../agent/model/agent-phase.enum';
import { StateStore } from '../../../state/store/state.store';
import { IssueState } from '../../../state/model/issue-state.model';
import { Project } from '../../model/project.model';

interface EventGroup {
    headingKey: string;
    events: AgentPhase[];
}

@Component({
    selector: 'app-workflow-event-state-map',
    templateUrl: './workflow-event-state-map.component.html',
    styleUrls: ['./workflow-event-state-map.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class WorkflowEventStateMapComponent implements OnInit, OnDestroy {
    public readonly project = input.required<Project>();

    private readonly workflowEventMapApi = inject(WorkflowEventMapApi);
    private readonly stateStore = inject(StateStore);
    private readonly formBuilder = inject(FormBuilder);
    private readonly toast = inject(ToastNotificationService);

    public readonly eventGroups: EventGroup[] = [
        {
            headingKey: 'WORKFLOW.STATE_MAP.GROUP_PR',
            events: [AgentPhase.Done]
        },
        {
            headingKey: 'WORKFLOW.STATE_MAP.GROUP_AGENT',
            events: [
                AgentPhase.Queued,
                AgentPhase.InProgress,
                AgentPhase.AwaitingInput,
                AgentPhase.AwaitingApproval,
                AgentPhase.PrOpen,
                AgentPhase.Failed,
                AgentPhase.Cancelled
            ]
        }
    ];

    private readonly events: AgentPhase[] = this.eventGroups.flatMap(group => group.events);
    public readonly eventLabels = PHASE_LABELS;
    public readonly isLoading = signal(true);

    private readonly rowStatus = signal<Record<number, UiSaveState>>({});

    public readonly states = signal<IssueState[]>([]);
    public mappingsFormArray!: FormArray<FormControl<number | null>>;

    private readonly subscription = new Subscription();

    public ngOnInit(): void {
        const idProject = this.project().idProject;

        // Deliberately not combined via combineLatest: states stay live for a sibling
        // panel, but rebuilding the form on every states emission would discard a pick the user hasn't saved yet.
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
            this.workflowEventMapApi.load$(this.project().idProject).subscribe({
                next: mappings => this.buildForm(mappings),
                error: () => {
                    this.isLoading.set(false);
                    this.toast.showError('WORKFLOW.STATE_MAP.LOAD_ERROR');
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

    private buildForm(mappings: WorkflowEventMappingEntry[]): void {
        const mappingByEvent = new Map(mappings.map(m => [m.event, m.idState]));
        this.mappingsFormArray = this.formBuilder.array(
            this.events.map(event =>
                this.formBuilder.control<number | null>(mappingByEvent.get(event) ?? null)
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

    public eventRowIndex(event: AgentPhase): number {
        return this.events.indexOf(event);
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
        const mappings: WorkflowEventMappingEntry[] = this.events
            .map((event, i) => ({
                event: event,
                idState: this.getMappingControl(i).value
            }))
            .filter(entry => entry.idState !== null);

        this.workflowEventMapApi.replace$(idProject, mappings).subscribe({
            next: () => this.setRowStatus(index, UiSaveState.Saved),
            error: () => {
                this.setRowStatus(index, UiSaveState.Error);
                this.toast.showError('WORKFLOW.STATE_MAP.SAVE_ERROR');
            }
        });
    }

    private setRowStatus(index: number, status: UiSaveState): void {
        this.rowStatus.update(m => ({ ...m, [index]: status }));
    }
}

import { FormControl, FormGroup, FormRecord } from '@angular/forms';
import { AgentEffort } from 'src/app/agent/model/agent-effort.enum';

export type AgentStageDefaultForm = FormGroup<{
    maxTurns: FormControl<number | null>;
    effort: FormControl<AgentEffort | null>;
}>;

export type AgentStageDefaultsForm = FormRecord<AgentStageDefaultForm>;

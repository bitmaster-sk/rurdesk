import { Validators } from '@angular/forms';

export const MAX_AGENT_TURNS = 10000;

export const MAX_TURNS_VALIDATORS = [Validators.min(1), Validators.max(MAX_AGENT_TURNS)];

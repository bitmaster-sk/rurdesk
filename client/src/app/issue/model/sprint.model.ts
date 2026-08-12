import { SprintState } from '../constants/sprint-state.enum';

export interface Sprint {
    idSprint: number;
    idProject: number;
    name: string;
    startAt: string;
    endAt: string;
    state: SprintState;
}

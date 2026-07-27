import { User } from 'src/app/auth/model/user.model';
import { SwimlaneCell } from './swimlane-cell.entity';

export interface SwimlaneRow {
    user: User | undefined; // undefined = Unassigned row
    cells: SwimlaneCell[];
}

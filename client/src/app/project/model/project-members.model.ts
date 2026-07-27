import { Role } from '../../shared/constants/role.enum';
import { User } from '../../auth/model/user.model';
import { Team } from '../../team/model/team.model';

export interface AroUser extends User {
    role: Role;
    isDirect: boolean;
    idsTeams: number[];
}

export interface AroTeam extends Team {
    role: Role;
    memberCount: number;
}

export interface ProjectMembersRes {
    users: AroUser[];
    teams: AroTeam[];
}

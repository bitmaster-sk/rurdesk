import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';
import { AgentRun } from 'src/app/agent/model/agent-run.model';
import { User } from 'src/app/auth/model/user.model';
import { SwimlaneCell } from 'src/app/issue/components/issue-kanban/entity/swimlane-cell.entity';
import { ExtendedIssue } from 'src/app/issue/model/extended-issue.model';
import { Issue } from 'src/app/issue/model/issue.model';
import { Skill } from 'src/app/shared/model/skill.model';

export abstract class Fixtures {
    public static issue(overrides: Partial<Issue> = {}): Issue {
        return {
            idIssue: 1,
            idIssuePublic: 1,
            idProject: 1,
            idState: null,
            idSeverity: null,
            idIssueType: null,
            title: 'Issue',
            description: '',
            tracked: 0,
            ...overrides
        };
    }

    public static extendedIssue(overrides: Partial<ExtendedIssue> = {}): ExtendedIssue {
        return {
            ...Fixtures.issue(overrides),
            state: undefined,
            severity: undefined,
            issueType: undefined,
            assignedToUser: undefined,
            ...overrides
        };
    }

    public static swimlaneCell(
        overrides: Partial<SwimlaneCell> & Pick<SwimlaneCell, 'state'>
    ): SwimlaneCell {
        return {
            user: undefined,
            tiles: [],
            total: 0,
            cursor: null,
            loading: false,
            ...overrides
        };
    }

    public static user(overrides: Partial<User> = {}): User {
        return {
            idUser: 1,
            name: 'Ada',
            email: 'ada@test.sk',
            colorAvatarBg: '#123456',
            isAgent: false,
            ...overrides
        };
    }

    public static agent(overrides: Partial<User> = {}): User {
        return Fixtures.user({
            idUser: 8,
            name: 'ci-agent',
            email: 'ci-agent@test.sk',
            isAgent: true,
            ...overrides
        });
    }

    public static skill(overrides: Partial<Skill> = {}): Skill {
        return {
            idSkill: 1,
            name: 'Verification rules',
            description: 'checks',
            content: 'body',
            isBuiltin: true,
            isEdited: false,
            createdAt: '2026-08-24T10:00:00Z',
            updatedAt: '2026-08-24T10:00:00Z',
            ...overrides
        };
    }

    public static agentRun(overrides: Partial<AgentRun> = {}): AgentRun {
        return {
            idRun: 55,
            idIssue: 10,
            idProject: 7,
            idUserAgent: 8,
            idGitIntegration: null,
            phase: AgentPhase.Queued,
            stagePlan: { stages: [] },
            queuePosition: null,
            prUrl: null,
            prHostType: null,
            prId: null,
            branchName: null,
            errorMessage: null,
            startedAt: null,
            finishedAt: null,
            createdAt: '2026-08-24T10:00:00Z',
            ...overrides
        };
    }
}

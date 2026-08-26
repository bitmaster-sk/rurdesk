import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';
import { AgentRun } from 'src/app/agent/model/agent-run.model';
import { User } from 'src/app/auth/model/user.model';
import { Skill } from 'src/app/shared/model/skill.model';

export abstract class Fixtures {
    public static user(overrides: Partial<User> = {}): User {
        return {
            idUser: 1,
            name: 'Ada',
            email: 'ada@test.sk',
            colorAvatarBg: '#123456',
            isBot: false,
            ...overrides
        };
    }

    public static bot(overrides: Partial<User> = {}): User {
        return Fixtures.user({
            idUser: 8,
            name: 'ci-bot',
            email: 'ci-bot@test.sk',
            isBot: true,
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
            idUserBot: 8,
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

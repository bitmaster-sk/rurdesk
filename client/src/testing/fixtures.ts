import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';
import { AgentEffort, AgentEffortSource } from 'src/app/agent/model/agent-effort.enum';
import { AgentRunStagePlanRes } from 'src/app/agent/model/agent-run-stage-plan.model';
import { AgentStageDefault } from 'src/app/agent/model/agent-stage-options.model';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { AgentRun, AgentStageProgress } from 'src/app/agent/model/agent-run.model';
import { User } from 'src/app/auth/model/user.model';
import { AgentStageOptionsByStage } from 'src/app/agent/model/agent-stage-options.model';
import { SwimlaneCell } from 'src/app/issue/components/issue-kanban/entity/swimlane-cell.entity';
import { ExtendedIssue } from 'src/app/issue/model/extended-issue.model';
import { Issue } from 'src/app/issue/model/issue.model';
import { Skill } from 'src/app/shared/model/skill.model';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { Tag } from 'src/app/tag/model/tag.model';

export abstract class Fixtures {
    public static agentStageDefaults(
        overrides: Partial<
            Pick<AgentRunStagePlanRes, 'defaultsByStage' | 'modelsWithoutThinking'>
        > = {}
    ): Pick<AgentRunStagePlanRes, 'defaultsByStage' | 'modelsWithoutThinking'> {
        const fromModel = { effort: null, source: AgentEffortSource.Model };
        return {
            modelsWithoutThinking: overrides.modelsWithoutThinking ?? [],
            defaultsByStage: {
                [AgentStage.Brainstorming]: { maxTurns: 250, effort: fromModel },
                [AgentStage.Design]: { maxTurns: 250, effort: fromModel },
                [AgentStage.ImplementationPlan]: { maxTurns: 250, effort: fromModel },
                [AgentStage.Implementation]: { maxTurns: 500, effort: fromModel },
                [AgentStage.ReleaseNotes]: { maxTurns: 250, effort: fromModel },
                ...overrides.defaultsByStage
            }
        };
    }

    public static appAgentStageDefaults(
        overrides: AgentStageOptionsByStage = {}
    ): AgentStageOptionsByStage {
        return {
            [AgentStage.Brainstorming]: { maxTurns: 250, effort: null },
            [AgentStage.Design]: { maxTurns: 250, effort: null },
            [AgentStage.ImplementationPlan]: { maxTurns: 250, effort: null },
            [AgentStage.Implementation]: { maxTurns: 500, effort: null },
            [AgentStage.ReleaseNotes]: { maxTurns: 250, effort: null },
            ...overrides
        };
    }

    public static stageDefault(
        maxTurns: number,
        effort: AgentEffort | null = null,
        source: AgentEffortSource = AgentEffortSource.Model
    ): AgentStageDefault {
        return { maxTurns, effort: { effort, source } };
    }

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
            idRelease: null,
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

    public static stageProgress(overrides: Partial<AgentStageProgress> = {}): AgentStageProgress {
        return {
            stage: 'brainstorming',
            status: 'pending',
            attempts: 0,
            tokensUsed: 0,
            toolCallsCount: 0,
            durationMs: 0,
            inputTokens: 0,
            outputTokens: 0,
            cacheReadTokens: 0,
            cacheWriteTokens: 0,
            costUsd: null,
            costEstimated: false,
            ...overrides
        };
    }

    public static state(overrides: Partial<IssueState> = {}): IssueState {
        return {
            idState: 1,
            idProject: 10,
            name: 'New',
            start: true,
            final: false,
            protected: false,
            orderRank: 0,
            ...overrides
        };
    }

    public static severity(overrides: Partial<IssueSeverity> = {}): IssueSeverity {
        return {
            idSeverity: 1,
            idProject: 10,
            title: 'High',
            color: '#d32f2f',
            protected: false,
            orderRank: 0,
            ...overrides
        };
    }

    public static issueType(overrides: Partial<IssueType> = {}): IssueType {
        return {
            idIssueType: 1,
            idProject: 10,
            name: 'Bug',
            protected: false,
            orderRank: 0,
            ...overrides
        };
    }

    public static tag(overrides: Partial<Tag> = {}): Tag {
        return {
            idTag: 1,
            idProject: 10,
            name: 'backend',
            color: '#2196f3',
            orderRank: 0,
            issueCount: 0,
            ...overrides
        };
    }
}

import {
    AgentStageCardBody,
    AgentStageCardPlanEntry,
    AgentStageCardPlanTags,
    AgentStageCardSource,
    AgentStageCardView
} from '../entity/agent-stage-card.entity';
import { AgentStageProgress, AgentStageStatus } from '../model/agent-run.model';
import { STAGE_LABELS } from '../model/agent-stage.enum';

const STAGE_STATUS_KEY: Record<AgentStageStatus, string> = {
    pending: 'AGENT.TIMELINE.STATUS.PENDING',
    active: 'AGENT.TIMELINE.STATUS.ACTIVE',
    done: 'AGENT.TIMELINE.STATUS.DONE',
    awaiting_approval: 'AGENT.TIMELINE.STATUS.AWAITING_APPROVAL',
    failed: 'AGENT.TIMELINE.STATUS.FAILED',
    skipped: 'AGENT.TIMELINE.STATUS.SKIPPED'
};

const STAGE_STATUS_GLYPH: Record<AgentStageStatus, string> = {
    pending: 'circle',
    active: 'loader-2',
    done: 'circle-check',
    awaiting_approval: 'clock',
    failed: 'alert-circle',
    skipped: 'circle-minus'
};

export abstract class AgentStageCardConverter {
    public static toViews(source: AgentStageCardSource): AgentStageCardView[] {
        const planByStage = new Map(source.plan.map(entry => [entry.name, entry]));
        const showAgent = AgentStageCardConverter.hasHandOff(source.stages);
        const views = source.stages.map(progress =>
            AgentStageCardConverter.toView(
                progress,
                planByStage.get(progress.stage) ?? null,
                source,
                showAgent
            )
        );
        const next = views.find(view => view.body === AgentStageCardBody.Plan);
        if (next) {
            next.isOpenByDefault = true;
        }
        return views;
    }

    private static toView(
        progress: AgentStageProgress,
        entry: AgentStageCardPlanEntry | null,
        source: AgentStageCardSource,
        showAgent: boolean
    ): AgentStageCardView {
        const isNotStarted = progress.status === 'pending' && !entry?.dispatched;
        return {
            stage: progress.stage,
            label: (STAGE_LABELS as Record<string, string>)[progress.stage] ?? progress.stage,
            status: progress.status,
            statusKey: STAGE_STATUS_KEY[progress.status] ?? progress.status,
            glyph: STAGE_STATUS_GLYPH[progress.status] ?? 'circle',
            agentName:
                showAgent && progress.idUserAgent != null
                    ? (source.agentNames.get(progress.idUserAgent) ?? null)
                    : null,
            at: progress.at ?? null,
            approvedAt: progress.approvedAt ?? null,
            attempts: progress.attempts,
            tokensUsed: progress.tokensUsed,
            toolCallsCount: progress.toolCallsCount,
            durationMs: progress.durationMs,
            model: progress.model ?? null,
            modelFallbackFrom: progress.modelFallbackFrom ?? null,
            maxTurns: progress.maxTurns ?? null,
            skillNames: (entry?.idsSkill ?? [])
                .map(idSkill => source.skillNames.get(idSkill))
                .filter((name): name is string => name !== undefined),
            planTags:
                isNotStarted && entry ? AgentStageCardConverter.planTags(entry, source) : null,
            body: AgentStageCardConverter.body(progress.status, isNotStarted, entry, source),
            isOpenByDefault: progress.status === 'awaiting_approval'
        };
    }

    private static body(
        status: AgentStageStatus,
        isNotStarted: boolean,
        entry: AgentStageCardPlanEntry | null,
        source: AgentStageCardSource
    ): AgentStageCardBody {
        if (status === 'skipped') {
            return AgentStageCardBody.None;
        }
        if (!isNotStarted) {
            return AgentStageCardBody.Ran;
        }
        return source.isEditable && entry ? AgentStageCardBody.Plan : AgentStageCardBody.None;
    }

    private static planTags(
        entry: AgentStageCardPlanEntry,
        source: AgentStageCardSource
    ): AgentStageCardPlanTags {
        const isModelOverride = !!entry.model && entry.model !== source.runModel;
        const isTurnsOverride =
            entry.maxTurns !== null && entry.maxTurns !== source.defaultMaxTurns(entry.name);
        return {
            skillsCount: entry.idsSkill.length,
            model: isModelOverride ? entry.model : null,
            maxTurns: isTurnsOverride ? entry.maxTurns : null
        };
    }

    private static hasHandOff(stages: AgentStageProgress[]): boolean {
        const agents = new Set(
            stages.map(stage => stage.idUserAgent).filter((id): id is number => id != null)
        );
        return agents.size > 1;
    }
}

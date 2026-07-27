import { describe, it, expect, vi, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { TranslateModule } from '@ngx-translate/core';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { UiModule } from 'src/app/ui/ui.module';

import { GanttArrowLayerComponent } from './gantt-arrow-layer.component';
import { GanttTimelineService } from '../../service/gantt-timeline.service';
import { GanttZoomLevel } from '../../constants/gantt-zoom-config';
import { ExtendedIssue } from '../../../../model/extended-issue.model';
import { ReadIssueRelationDto } from '../../../../model/issue-relation.model';
import { IssueRelationType } from '../../../../constants/issue-relation-type.enum';
import { IssueRelationSubType } from '../../../../constants/issue-relation-subtype.enum';
import { HandleSide } from '../../constants/gantt-handle-side.enum';

function makeTask(over: Partial<ExtendedIssue> = {}): ExtendedIssue {
    return {
        idIssue: 1,
        idIssuePublic: 1,
        idProject: 10,
        title: 'Task',
        description: '',
        tracked: 0,
        idState: null,
        idSeverity: null,
        scheduledAt: new Date('2025-01-15T00:00:00Z'),
        estimated: 3600,
        state: undefined,
        severity: undefined,
        ...over
    } as ExtendedIssue;
}

const rowHeightSignal = Object.assign(() => 72, { set: vi.fn() });

const timelineServiceMock = {
    zoomLevel: vi.fn(() => GanttZoomLevel.Week),
    setZoom: vi.fn(),
    rowHeight: rowHeightSignal,
    toPixel: vi.fn((date: Date) => {
        // Simple linear: 100px per day from Jan 1
        return (
            ((date.getTime() - new Date('2025-01-01T00:00:00Z').getTime()) /
                (24 * 60 * 60 * 1000)) *
            100
        );
    }),
    toDate: vi.fn(() => new Date('2025-01-15'))
};

function makeRelation(over: Partial<ReadIssueRelationDto> = {}): ReadIssueRelationDto {
    return {
        idIssueRelation: 1,
        direction: 'outbound',
        relationType: IssueRelationType.Schedule,
        relationSubType: IssueRelationSubType.FinishToStart,
        from: { idIssuePublic: 1 },
        to: { idIssuePublic: 2 },
        lagMinutes: null,
        ...over
    } as ReadIssueRelationDto;
}

async function createFixture(
    overrides: {
        tasks?: ExtendedIssue[];
        relations?: ReadIssueRelationDto[];
    } = {}
) {
    const tasks = overrides.tasks ?? [
        makeTask({
            idIssuePublic: 1,
            scheduledAt: new Date('2025-01-15T00:00:00Z'),
            estimated: 3600
        }),
        makeTask({
            idIssuePublic: 2,
            scheduledAt: new Date('2025-01-16T00:00:00Z'),
            estimated: 3600
        })
    ];
    const relations = overrides.relations ?? [makeRelation()];

    TestBed.configureTestingModule({
        imports: [TranslateModule.forRoot(), UiModule],
        declarations: [GanttArrowLayerComponent],
        providers: [
            { provide: GanttTimelineService, useValue: timelineServiceMock },
            provideNoopAnimations()
        ]
    });

    await TestBed.compileComponents();
    const fixture = TestBed.createComponent(GanttArrowLayerComponent);
    const comp = fixture.componentInstance;
    fixture.componentRef.setInput('tasks', tasks);
    fixture.componentRef.setInput('relations', relations);
    fixture.componentRef.setInput('totalWidth', 2000);
    fixture.componentRef.setInput('totalHeight', 500);
    // Set viewport to cover the tasks (pixel ~1400-1500)
    fixture.componentRef.setInput('scrollLeft', 1000);
    fixture.componentRef.setInput('viewportWidth', 1000);
    fixture.componentRef.setInput('viewportHeight', 500);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, comp };
}

describe('GanttArrowLayerComponent (TestBed)', () => {
    let comp: any;
    let fixture: any;

    beforeEach(async () => {
        const result = await createFixture();
        comp = result.comp;
        fixture = result.fixture;
    });

    // =========================================================================
    // arrows computed
    // =========================================================================

    describe('arrows', () => {
        it('generates arrow paths for outbound relations', () => {
            const arrows = comp.arrows();
            expect(arrows.length).toBe(1);
            expect(arrows[0].relationId).toBe(1);
            expect(arrows[0].path).toMatch(/^M/);
        });

        it('skips inbound relations', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ direction: 'inbound' })]
            });
            expect(result.comp.arrows()).toHaveLength(0);
        });

        it('skips relations where task not found', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [
                    makeRelation({
                        from: { idIssuePublic: 999 },
                        to: { idIssuePublic: 2 }
                    })
                ]
            });
            expect(result.comp.arrows()).toHaveLength(0);
        });

        it('skips unscheduled tasks', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                tasks: [
                    makeTask({ idIssuePublic: 1, scheduledAt: null }),
                    makeTask({ idIssuePublic: 2, scheduledAt: new Date('2025-01-16T00:00:00Z') })
                ],
                relations: [makeRelation()]
            });
            expect(result.comp.arrows()).toHaveLength(0);
        });

        it('generates correct lag label for minutes < 60', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ lagMinutes: 30 })]
            });
            expect(result.comp.arrows()[0].lagLabel).toBe('30m');
        });

        it('generates correct lag label for hours', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ lagMinutes: 90 })]
            });
            expect(result.comp.arrows()[0].lagLabel).toBe('1h30m');
        });

        it('generates correct lag label for exact hours', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ lagMinutes: 120 })]
            });
            expect(result.comp.arrows()[0].lagLabel).toBe('2h');
        });

        it('lagLabel is null when no lag', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ lagMinutes: null })]
            });
            expect(result.comp.arrows()[0].lagLabel).toBeNull();
        });

        it('isDirectional true for FinishToStart', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationSubType: IssueRelationSubType.FinishToStart })]
            });
            expect(result.comp.arrows()[0].isDirectional).toBe(true);
        });

        it('isDirectional false for StartToStart', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationSubType: IssueRelationSubType.StartToStart })]
            });
            expect(result.comp.arrows()[0].isDirectional).toBe(false);
        });

        it('isDirectional true for StartToFinish', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationSubType: IssueRelationSubType.StartToFinish })]
            });
            expect(result.comp.arrows()[0].isDirectional).toBe(true);
        });

        it('isDrawIn true when drawInRelation matches', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture();
            result.fixture.componentRef.setInput('drawInRelation', { from: 1, to: 2 });
            result.fixture.detectChanges();
            expect(result.comp.arrows()[0].isDrawIn).toBe(true);
        });

        it('isDrawIn false when drawInRelation does not match', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture();
            result.fixture.componentRef.setInput('drawInRelation', { from: 5, to: 6 });
            result.fixture.detectChanges();
            expect(result.comp.arrows()[0].isDrawIn).toBe(false);
        });

        it('traceDelayMs set during critical tracing', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ idIssueRelation: 10 })]
            });
            result.fixture.componentRef.setInput('isCriticalTracing', true);
            const order = new Map([[10, 2]]);
            result.fixture.componentRef.setInput('criticalRelationOrder', order);
            result.fixture.detectChanges();
            expect(result.comp.arrows()[0].traceDelayMs).toBe(240);
        });

        it('traceDelayMs null when not tracing', () => {
            expect(comp.arrows()[0].traceDelayMs).toBeNull();
        });

        it('extracts idProject from first task', () => {
            expect(comp.arrows()[0].idProject).toBe(10);
        });

        it('idProject is 0 when tasks empty', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({ tasks: [], relations: [] });
            expect(result.comp.arrows()).toHaveLength(0);
        });

        it('relationTypeTranslationKey for schedule', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationType: IssueRelationType.Schedule })]
            });
            expect(result.comp.arrows()[0].relationTypeTranslationKey).toBe('RELATION.SCHEDULE');
        });

        it('relationTypeTranslationKey for hierarchy', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationType: IssueRelationType.Hierarchy })]
            });
            expect(result.comp.arrows()[0].relationTypeTranslationKey).toBe('RELATION.HIERARCHY');
        });

        it('relationTypeTranslationKey for duplicates', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationType: IssueRelationType.Duplicates })]
            });
            expect(result.comp.arrows()[0].relationTypeTranslationKey).toBe('RELATION.DUPLICATES');
        });

        it('relationSubTypeTranslationKey for finish-to-start', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationSubType: IssueRelationSubType.FinishToStart })]
            });
            expect(result.comp.arrows()[0].relationSubTypeTranslationKey).toBe(
                'RELATION.FINISH_TO_START'
            );
        });

        it('relationSubTypeTranslationKey for start-to-start', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationSubType: IssueRelationSubType.StartToStart })]
            });
            expect(result.comp.arrows()[0].relationSubTypeTranslationKey).toBe(
                'RELATION.START_TO_START'
            );
        });

        it('relationSubTypeTranslationKey for finish-to-finish', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationSubType: IssueRelationSubType.FinishToFinish })]
            });
            expect(result.comp.arrows()[0].relationSubTypeTranslationKey).toBe(
                'RELATION.FINISH_TO_FINISH'
            );
        });

        it('relationSubTypeTranslationKey for start-to-finish', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationSubType: IssueRelationSubType.StartToFinish })]
            });
            expect(result.comp.arrows()[0].relationSubTypeTranslationKey).toBe(
                'RELATION.START_TO_FINISH'
            );
        });

        it('relationSubTypeTranslationKey for parent', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationSubType: IssueRelationSubType.Parent })]
            });
            expect(result.comp.arrows()[0].relationSubTypeTranslationKey).toBe('RELATION.PARENT');
        });

        it('relationSubTypeTranslationKey for child', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationSubType: IssueRelationSubType.Child })]
            });
            expect(result.comp.arrows()[0].relationSubTypeTranslationKey).toBe('RELATION.CHILD');
        });

        it('relationSubTypeTranslationKey default for unknown', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationSubType: 'unknown' as any })]
            });
            expect(result.comp.arrows()[0].relationSubTypeTranslationKey).toBe('RELATION.BASIC');
        });

        it('relationTypeTranslationKey default for unknown', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                relations: [makeRelation({ relationType: 'unknown' as any })]
            });
            expect(result.comp.arrows()[0].relationTypeTranslationKey).toBe('RELATION.BASIC');
        });
    });

    // =========================================================================
    // displayArrows — hover/selected ordering
    // =========================================================================

    describe('displayArrows', () => {
        it('returns arrows unchanged when no hover/selected', () => {
            const arrows = comp.displayArrows();
            expect(arrows.length).toBe(1);
        });

        it('places selected arrow last (on top)', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                tasks: [
                    makeTask({ idIssuePublic: 1 }),
                    makeTask({ idIssuePublic: 2 }),
                    makeTask({ idIssuePublic: 3 })
                ],
                relations: [
                    makeRelation({ idIssueRelation: 1 }),
                    makeRelation({ idIssueRelation: 2, to: { idIssuePublic: 3 } })
                ]
            });
            result.fixture.componentRef.setInput('selectedRelationId', 1);
            result.fixture.detectChanges();
            const arrows = result.comp.displayArrows();
            expect(arrows[arrows.length - 1].relationId).toBe(1);
        });

        it('places hovered arrow above selected', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                tasks: [
                    makeTask({ idIssuePublic: 1 }),
                    makeTask({ idIssuePublic: 2 }),
                    makeTask({ idIssuePublic: 3 })
                ],
                relations: [
                    makeRelation({ idIssueRelation: 1 }),
                    makeRelation({ idIssueRelation: 2, to: { idIssuePublic: 3 } })
                ]
            });
            result.fixture.componentRef.setInput('selectedRelationId', 1);
            result.comp.onArrowMouseEnter(2);
            result.fixture.detectChanges();
            const arrows = result.comp.displayArrows();
            expect(arrows[arrows.length - 1].relationId).toBe(2);
        });
    });

    // =========================================================================
    // drawingArrowPath
    // =========================================================================

    describe('drawingArrowPath', () => {
        it('returns null when no drawingLine', () => {
            expect(comp.drawingArrowPath()).toBeNull();
        });

        it('returns a path string when drawingLine is set', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture();
            result.fixture.componentRef.setInput('drawingLine', {
                sourceTaskId: 1,
                sourceSide: HandleSide.Right,
                clientX: 500,
                clientY: 100
            });
            result.fixture.detectChanges();
            const path = result.comp.drawingArrowPath();
            expect(path).toMatch(/^M/);
        });

        it('returns null when source task not found', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture();
            result.fixture.componentRef.setInput('drawingLine', {
                sourceTaskId: 999,
                sourceSide: HandleSide.Right,
                clientX: 500,
                clientY: 100
            });
            result.fixture.detectChanges();
            expect(result.comp.drawingArrowPath()).toBeNull();
        });

        it('snaps to drop target handle when dropTarget is set', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture();
            result.fixture.componentRef.setInput('drawingLine', {
                sourceTaskId: 1,
                sourceSide: HandleSide.Right,
                clientX: 500,
                clientY: 100
            });
            result.fixture.componentRef.setInput('dropTarget', {
                taskId: 2,
                side: HandleSide.Left
            });
            result.fixture.detectChanges();
            const path = result.comp.drawingArrowPath();
            expect(path).toMatch(/^M.*L/);
        });
    });

    // =========================================================================
    // onArrowClick / onArrowMouseEnter / onArrowMouseLeave
    // =========================================================================

    describe('interaction handlers', () => {
        it('onArrowClick emits arrowClicked', () => {
            let emitted: number | null = null;
            comp.arrowClicked.subscribe((id: number) => (emitted = id));
            comp.onArrowClick(42);
            expect(emitted).toBe(42);
        });

        it('onArrowMouseEnter sets hovered relation', () => {
            comp.onArrowMouseEnter(5);
            expect(comp.idHoveredRelation$()).toBe(5);
        });

        it('onArrowMouseLeave clears hovered relation', () => {
            comp.onArrowMouseEnter(5);
            comp.onArrowMouseLeave();
            expect(comp.idHoveredRelation$()).toBeNull();
        });
    });

    // =========================================================================
    // viewport culling
    // =========================================================================

    describe('viewport culling', () => {
        it('skips arrows where both endpoints are outside viewport', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture();
            // Set viewport far away from the tasks
            result.fixture.componentRef.setInput('scrollLeft', 50000);
            result.fixture.componentRef.setInput('viewportWidth', 1000);
            result.fixture.detectChanges();
            expect(result.comp.arrows()).toHaveLength(0);
        });

        it('keeps arrows when critical tracing is active (no culling)', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture();
            result.fixture.componentRef.setInput('scrollLeft', 50000);
            result.fixture.componentRef.setInput('viewportWidth', 1000);
            result.fixture.componentRef.setInput('isCriticalTracing', true);
            result.fixture.detectChanges();
            expect(result.comp.arrows()).toHaveLength(1);
        });
    });

    // =========================================================================
    // lane assignment — multiple arrows from same side
    // =========================================================================

    describe('lane assignment', () => {
        it('assigns different exit lanes to arrows from the same source side', async () => {
            TestBed.resetTestingModule();
            const result = await createFixture({
                tasks: [
                    makeTask({ idIssuePublic: 1 }),
                    makeTask({ idIssuePublic: 2 }),
                    makeTask({ idIssuePublic: 3 })
                ],
                relations: [
                    makeRelation({
                        idIssueRelation: 1,
                        from: { idIssuePublic: 1 },
                        to: { idIssuePublic: 2 }
                    }),
                    makeRelation({
                        idIssueRelation: 2,
                        from: { idIssuePublic: 1 },
                        to: { idIssuePublic: 3 }
                    })
                ]
            });
            result.fixture.detectChanges();
            const arrows = result.comp.arrows();
            // Both arrows share the same exit (from task 1, right side)
            // They should have different paths (lanes prevent overlap)
            expect(arrows).toHaveLength(2);
            expect(arrows[0].path).not.toBe(arrows[1].path);
        });
    });
});

import {
    IssuesFilter,
    IssuesFilterParams
} from '../../issue/components/filter/issue-filter.entity';
import { SavedViewConfig, SavedViewKanbanLayout } from './saved-view.model';

const DATE_KEYS = ['createAtFrom', 'createAtTo', 'updateAtFrom', 'updateAtTo'] as const;
const ID_KEYS = ['idsSeverity', 'idsIssueType', 'idsState', 'idsAssignedTo'] as const;

export const SAVED_VIEW_DEFAULT_ORDER = {
    orderColumn: 'updateAt',
    orderDirection: 'desc'
} as const;

/**
 * Persisting these would freeze something the user expects to keep moving: the kanban sprint
 * tab, ad-hoc task selections, and the calendar/gantt windows recomputed from "now".
 */
const SAVED_VIEW_EXCLUDED: readonly (keyof IssuesFilterParams)[] = [
    'idProject',
    'idsIssuePublic',
    'idSprint',
    'sprintUnset',
    'scheduledAtFrom',
    'scheduledAtTo'
];

export abstract class SavedViewConfigConverter {
    public static toFilter(config: SavedViewConfig): Omit<IssuesFilter, 'idProject'> {
        const { v: _v, kanbanLayout: _kanbanLayout, orderColumn, orderDirection, ...rest } = config;
        const params = { ...rest } as unknown as IssuesFilterParams;
        const writable = params as Record<string, unknown>;
        for (const key of DATE_KEYS) {
            const parsed = config[key] ? new Date(config[key]) : null;
            if (parsed && !Number.isNaN(parsed.getTime())) {
                writable[key] = parsed;
            } else {
                delete writable[key];
            }
        }
        for (const key of ID_KEYS) {
            if (key in config && !Array.isArray(config[key])) {
                delete writable[key];
            }
        }
        return {
            ...params,
            orderColumn: orderColumn ?? SAVED_VIEW_DEFAULT_ORDER.orderColumn,
            orderDirection: orderDirection ?? SAVED_VIEW_DEFAULT_ORDER.orderDirection
        };
    }

    public static toConfig(
        filter: IssuesFilter,
        kanbanLayout?: SavedViewKanbanLayout
    ): SavedViewConfig {
        const config: SavedViewConfig = { v: 1 };
        const writable = config as unknown as Record<string, unknown>;
        for (const [key, value] of Object.entries(filter)) {
            if (value === undefined || value === null) {
                continue;
            }
            if ((SAVED_VIEW_EXCLUDED as readonly string[]).includes(key)) {
                continue;
            }
            writable[key] = value instanceof Date ? value.toISOString() : value;
        }
        if (kanbanLayout) {
            config.kanbanLayout = kanbanLayout;
        }
        return config;
    }
}

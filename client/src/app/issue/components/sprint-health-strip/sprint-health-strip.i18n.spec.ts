import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

const COUNTED_KEYS = [
    'HEALTH_DAYS_LEFT',
    'HEALTH_STARTS_IN',
    'UNIT_POINTS_SHORT',
    'UNIT_TASKS_SHORT'
];

const REQUIRED_KEYS = [
    'HEALTH_DAY_X_OF_Y',
    'HEALTH_CLOSED_TAG',
    'HEALTH_PACE',
    'HEALTH_ON_TRACK',
    'HEALTH_BEHIND',
    'HEALTH_TOO_EARLY',
    'HEALTH_OVER_COMMITTED',
    'HEALTH_PLANNED',
    'HEALTH_PROGRESS',
    'HEALTH_DONE_ONLY',
    'HEALTH_NO_POINTS',
    'HEALTH_BACKLOG_SUMMARY',
    'UNIT_POINTS',
    'UNIT_TASKS'
];

function sourceFiles(dir: string): string[] {
    return readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
        const path = join(dir, entry.name);
        if (entry.isDirectory()) {
            return sourceFiles(path);
        }
        const isSource = path.endsWith('.ts') || path.endsWith('.html');
        return isSource && !path.endsWith('.spec.ts') ? [path] : [];
    });
}

describe('sprint health translations', () => {
    const en = JSON.parse(readFileSync('src/assets/i18n/en.json', 'utf8'));
    const sprints = en.ISSUE.KANBAN.SPRINTS;
    const source = sourceFiles('src/app')
        .map(path => readFileSync(path, 'utf8'))
        .join('\n');

    it.each(REQUIRED_KEYS)('defines %s', key => {
        expect(sprints[key]).toBeTypeOf('string');
        expect(sprints[key].length).toBeGreaterThan(0);
    });

    it.each(COUNTED_KEYS)('gives %s a SINGULAR and a PLURAL that differ', key => {
        expect(sprints[key].SINGULAR).toBeTypeOf('string');
        expect(sprints[key].PLURAL).toBeTypeOf('string');
        expect(sprints[key].SINGULAR).not.toBe(sprints[key].PLURAL);
    });

    it.each(
        Object.keys(sprints).filter(key => key.startsWith('HEALTH_') || key.startsWith('UNIT_'))
    )('reaches %s from the app, so no dead string ships', key => {
        expect(source).toMatch(new RegExp(`${key}(?![A-Z_])`));
    });

    it('parameterises every unit-bearing string', () => {
        for (const key of [
            'HEALTH_PLANNED',
            'HEALTH_PROGRESS',
            'HEALTH_BEHIND',
            'HEALTH_DONE_ONLY'
        ]) {
            expect(sprints[key]).toContain('{{unit}}');
        }
    });
});

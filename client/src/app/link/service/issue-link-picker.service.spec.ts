import { Injector, runInInjectionContext } from '@angular/core';
import { Subject } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { IssueLinkApi } from '../api/issue-link.api.service';
import { IssueLinkOption } from '../model/issue-link.model';
import { IssueLinkPicker } from './issue-link-picker.service';
import { LinkScope } from './link-scope.service';

const options: IssueLinkOption[] = [
    { idProject: 7, projectName: 'Rurdesk', idIssuePublic: 123, title: 'Fix login' },
    { idProject: 9, projectName: 'Website', idIssuePublic: 12, title: 'Blog redesign' }
];

function buildPicker(idProject: number | null = null): {
    picker: IssueLinkPicker;
    load: ReturnType<typeof vi.fn>;
} {
    const load = vi.fn().mockImplementation(() => new Subject<IssueLinkOption[]>());
    const scope = new LinkScope();
    scope.idProject.set(idProject);
    const injector = Injector.create({
        providers: [
            { provide: IssueLinkApi, useValue: { load$: load } },
            { provide: LinkScope, useValue: scope }
        ]
    });
    return { picker: runInInjectionContext(injector, () => new IssueLinkPicker()), load };
}

function answer(load: ReturnType<typeof vi.fn>, call: number, result: IssueLinkOption[]): void {
    (load.mock.results[call].value as Subject<IssueLinkOption[]>).next(result);
}

describe('IssueLinkPicker', () => {
    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    it('opens after a # at a word start and searches with the current project first', () => {
        const { picker, load } = buildPicker(7);

        expect(picker.detect('fixed in #lo')).toEqual({ start: 9, query: 'lo' });
        vi.advanceTimersByTime(IssueLinkPicker.searchDelay);

        expect(load).toHaveBeenCalledWith('lo', 7);
        expect(picker.isVisible()).toBe(true);
        expect(picker.isLoading()).toBe(true);
    });

    it('does not open for a heading or a hash inside a word', () => {
        const { picker } = buildPicker();

        expect(picker.detect('# Title')).toBeNull();
        expect(picker.detect('C#')).toBeNull();
        expect(picker.isVisible()).toBe(false);
    });

    it('shows only the results of the text that is typed now', () => {
        const { picker, load } = buildPicker();
        picker.detect('#1');
        vi.advanceTimersByTime(IssueLinkPicker.searchDelay);
        answer(load, 0, options);
        expect(picker.options()).toEqual(options);

        picker.detect('#12');

        expect(picker.options()).toEqual([]);
        expect(picker.onKey('Enter')).toEqual({ isHandled: false });
        vi.advanceTimersByTime(IssueLinkPicker.searchDelay);
        answer(load, 1, [options[1]]);
        expect(picker.options()).toEqual([options[1]]);
    });

    it('moves through the options and picks the highlighted one', () => {
        const { picker, load } = buildPicker();
        picker.detect('#');
        vi.advanceTimersByTime(IssueLinkPicker.searchDelay);
        answer(load, 0, options);

        expect(picker.onKey('ArrowDown')).toEqual({ isHandled: true });
        expect(picker.onKey('ArrowDown')).toEqual({ isHandled: true });
        expect(picker.onKey('Tab')).toEqual({ isHandled: true, picked: options[1] });
        expect(picker.onKey('ArrowUp')).toEqual({ isHandled: true });
        expect(picker.onKey('Enter')).toEqual({ isHandled: true, picked: options[0] });
    });

    it('starts at the first option again when the search changes', () => {
        const { picker, load } = buildPicker();
        picker.detect('#1');
        vi.advanceTimersByTime(IssueLinkPicker.searchDelay);
        answer(load, 0, options);
        picker.onKey('ArrowDown');

        picker.detect('#12');
        vi.advanceTimersByTime(IssueLinkPicker.searchDelay);
        answer(load, 1, [options[1]]);

        expect(picker.onKey('Enter')).toEqual({ isHandled: true, picked: options[1] });
    });

    it('closes on Escape and when the # is gone', () => {
        const { picker, load } = buildPicker();
        picker.detect('#');
        vi.advanceTimersByTime(IssueLinkPicker.searchDelay);
        answer(load, 0, options);

        expect(picker.onKey('Escape')).toEqual({ isHandled: true });
        expect(picker.isVisible()).toBe(false);

        picker.detect('#');
        picker.detect('no hash');
        expect(picker.isVisible()).toBe(false);
        expect(picker.onKey('Enter')).toEqual({ isHandled: false });
    });
});

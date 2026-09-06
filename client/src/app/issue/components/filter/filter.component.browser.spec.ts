import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { BehaviorSubject, EMPTY, of } from 'rxjs';
import { ProjectMemberStore } from '../../../project/project-member.store';
import { ProjectStore } from '../../../project/project.store';
import { SeverityStore } from '../../../severity/store/severity.store';
import { StateStore } from '../../../state/store/state.store';
import { FilterComponent } from './filter.component';
import { IssuesFilter, IssuesFilterParams } from './issue-filter.entity';
import { IssueFilterStore } from './issue-filter.store';

// Both directions of the value ↔ params mapping are hand-written — the spot where
// a filter silently goes missing.
describe('FilterComponent — date modes (browser)', () => {
    let setFilter: ReturnType<typeof vi.fn>;
    let initialFilter$: BehaviorSubject<IssuesFilter>;

    const filter = (extra: Partial<IssuesFilterParams> = {}): IssuesFilter => ({
        idProject: 1,
        orderColumn: 'idIssuePublic',
        orderDirection: 'desc',
        ...extra
    });

    beforeEach(async () => {
        setFilter = vi.fn();
        initialFilter$ = new BehaviorSubject<IssuesFilter>(filter());

        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [FilterComponent],
            providers: [
                {
                    provide: IssueFilterStore,
                    useValue: { setFilter, initialFilter$, getFilter: () => null }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } },
                { provide: ProjectMemberStore, useValue: { users$: of([]) } },
                { provide: SeverityStore, useValue: { severitiesByProject$: () => of([]) } },
                { provide: StateStore, useValue: { statesByProject$: () => of([]) } },
                { provide: I18nService, useValue: { instant: (key: string) => key } }
            ]
        })
            .overrideComponent(FilterComponent, { set: { template: '' } })
            .compileComponents();
    });

    function render() {
        const fixture = TestBed.createComponent(FilterComponent);
        fixture.detectChanges();
        return fixture.componentInstance;
    }

    function presetValues(component: FilterComponent): string[] {
        return (component as unknown as { updateAtPresets: () => { value: string }[] })
            .updateAtPresets()
            .map(preset => preset.value);
    }

    function lastFilter(): IssuesFilterParams {
        return setFilter.mock.calls[setFilter.mock.calls.length - 1][0] as IssuesFilterParams;
    }

    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    it('sends the selected preset as a rolling window, with no absolute bounds', () => {
        const component = render();
        component.form.patchValue({ updateAt: { preset: '30d' } });

        expect(lastFilter().updateAtWithin).toBe('30d');
        expect(lastFilter().updateAtFrom).toBeNull();
        expect(lastFilter().updateAtTo).toBeNull();
    });

    it('sends the datepicker range and no window in custom mode', () => {
        const component = render();
        const from = new Date('2026-01-01T00:00:00Z');
        const to = new Date('2026-01-31T00:00:00Z');
        component.form.patchValue({ createAt: { from, to } });

        expect(lastFilter().createAtFrom).toBe(from);
        expect(lastFilter().createAtTo).toBe(to);
        expect(lastFilter().createAtWithin).toBeNull();
    });

    it('sends no date filter in the default "any" mode', () => {
        const component = render();
        component.form.patchValue({ title: 'anything' });
        vi.advanceTimersByTime(300);

        expect(lastFilter().updateAtWithin).toBeNull();
        expect(lastFilter().updateAtFrom).toBeNull();
    });

    it('keeps the two date fields independent', () => {
        const component = render();
        component.form.patchValue({ createAt: { preset: '7d' }, updateAt: { preset: '90d' } });

        expect(lastFilter().createAtWithin).toBe('7d');
        expect(lastFilter().updateAtWithin).toBe('90d');
    });

    it('hydrates a preset value from an incoming window', () => {
        initialFilter$.next(filter({ updateAtWithin: '30d' }));
        const component = render();

        expect(component.form.get('updateAt')!.value).toEqual({ preset: '30d' });
    });

    it('hydrates a range value from incoming absolute bounds', () => {
        const from = new Date('2026-01-01T00:00:00Z');
        initialFilter$.next(filter({ createAtFrom: from }));
        const component = render();

        expect(component.form.get('createAt')!.value).toEqual({ from, to: undefined });
    });

    it('hydrates null when the incoming filter has no date bounds', () => {
        const component = render();
        expect(component.form.get('createAt')!.value).toBeNull();
    });

    // A saved view or MCP filter can carry a duration the presets do not list.
    it('adds a preset row for a window the defaults do not cover', () => {
        initialFilter$.next(filter({ updateAtWithin: '1d8h6m' }));
        const component = render();

        expect(component.form.get('updateAt')!.value).toEqual({ preset: '1d8h6m' });
        expect(presetValues(component)).toContain('1d8h6m');
    });

    it('does not duplicate a row that is already a preset', () => {
        initialFilter$.next(filter({ updateAtWithin: '7d' }));
        const component = render();

        expect(presetValues(component).filter(value => value === '7d')).toHaveLength(1);
    });

    it('hydration does not push a filter back into the store', () => {
        initialFilter$.next(filter({ updateAtWithin: '30d' }));
        render();

        expect(setFilter).not.toHaveBeenCalled();
    });
});

// The board and gantt create the panel on demand, by when initialFilter$ replays nothing.
describe('FilterComponent — late mount (browser)', () => {
    const activeFilter: IssuesFilter = {
        idProject: 1,
        idsState: [4],
        updateAtWithin: '30d',
        orderColumn: 'idIssuePublic',
        orderDirection: 'desc'
    };

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [FilterComponent],
            providers: [
                {
                    provide: IssueFilterStore,
                    useValue: {
                        setFilter: vi.fn(),
                        // A store whose latest emission was NOT initial: nothing replays.
                        initialFilter$: EMPTY,
                        getFilter: () => activeFilter
                    }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } },
                { provide: ProjectMemberStore, useValue: { users$: of([]) } },
                { provide: SeverityStore, useValue: { severitiesByProject$: () => of([]) } },
                { provide: StateStore, useValue: { statesByProject$: () => of([]) } },
                { provide: I18nService, useValue: { instant: (key: string) => key } }
            ]
        })
            .overrideComponent(FilterComponent, { set: { template: '' } })
            .compileComponents();
    });

    it('hydrates from the live filter when initialFilter$ replays nothing', () => {
        const fixture = TestBed.createComponent(FilterComponent);
        fixture.detectChanges();

        expect(fixture.componentInstance.form.get('idsState')!.value).toEqual([4]);
        expect(fixture.componentInstance.form.get('updateAt')!.value).toEqual({ preset: '30d' });
    });
});

describe('FilterComponent — rehydration after mount (browser)', () => {
    let setFilter: ReturnType<typeof vi.fn>;
    let initialFilter$: BehaviorSubject<IssuesFilter>;

    const filter = (extra: Partial<IssuesFilterParams> = {}): IssuesFilter => ({
        idProject: 1,
        orderColumn: 'idIssuePublic',
        orderDirection: 'desc',
        ...extra
    });

    beforeEach(async () => {
        setFilter = vi.fn();
        initialFilter$ = new BehaviorSubject<IssuesFilter>(filter({ idsState: [1] }));

        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [FilterComponent],
            providers: [
                {
                    provide: IssueFilterStore,
                    useValue: { setFilter, initialFilter$, getFilter: () => null }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } },
                { provide: ProjectMemberStore, useValue: { users$: of([]) } },
                { provide: SeverityStore, useValue: { severitiesByProject$: () => of([]) } },
                { provide: StateStore, useValue: { statesByProject$: () => of([]) } },
                { provide: I18nService, useValue: { instant: (key: string) => key } }
            ]
        })
            .overrideComponent(FilterComponent, { set: { template: '' } })
            .compileComponents();
    });

    function render(): FilterComponent {
        const fixture = TestBed.createComponent(FilterComponent);
        fixture.detectChanges();
        return fixture.componentInstance;
    }

    it('follows a new initial filter set after mount', () => {
        const component = render();
        expect(component.form.get('idsState')!.value).toEqual([1]);

        initialFilter$.next(filter({ idsState: [2, 3], stateUnset: true }));

        expect(component.form.get('idsState')!.value).toEqual([2, 3]);
        expect(component.form.get('stateUnset')!.value).toBe(true);
    });

    // Regression: the panel owned an idProject control nothing bound to, so on a late mount
    // the first filter edit pushed idProject: null → GET /project/null/issue → 400.
    it('never pushes idProject into the store', () => {
        const component = render();

        component.form.patchValue({ idsAssignedTo: [1] });

        expect(setFilter).toHaveBeenCalled();
        expect('idProject' in setFilter.mock.calls[0][0]).toBe(false);
    });

    it('rehydration never loops back into the store', () => {
        render();
        initialFilter$.next(filter({ idsState: [2, 3] }));

        expect(setFilter).not.toHaveBeenCalled();
    });

    it('re-seeds the preset rows for a window the defaults do not cover', () => {
        const component = render();
        const presets = (): string[] =>
            (component as unknown as { updateAtPresets: () => { value: string }[] })
                .updateAtPresets()
                .map(preset => preset.value);
        expect(presets()).not.toContain('1d8h6m');

        initialFilter$.next(filter({ updateAtWithin: '1d8h6m' }));

        expect(component.form.get('updateAt')!.value).toEqual({ preset: '1d8h6m' });
        expect(presets()).toContain('1d8h6m');
    });

    it('clears a date field when the applied view has no window', () => {
        const component = render();
        initialFilter$.next(filter({ updateAtWithin: '30d' }));
        expect(component.form.get('updateAt')!.value).toEqual({ preset: '30d' });

        initialFilter$.next(filter({}));

        expect(component.form.get('updateAt')!.value).toBeNull();
    });
});

describe('FilterComponent — title debounce (browser)', () => {
    let setFilter: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        setFilter = vi.fn();

        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [FilterComponent],
            providers: [
                {
                    provide: IssueFilterStore,
                    useValue: { setFilter, initialFilter$: EMPTY, getFilter: () => null }
                },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } },
                { provide: ProjectMemberStore, useValue: { users$: of([]) } },
                { provide: SeverityStore, useValue: { severitiesByProject$: () => of([]) } },
                { provide: StateStore, useValue: { statesByProject$: () => of([]) } },
                { provide: I18nService, useValue: { instant: (key: string) => key } }
            ]
        })
            .overrideComponent(FilterComponent, { set: { template: '' } })
            .compileComponents();
    });

    function render(): FilterComponent {
        const fixture = TestBed.createComponent(FilterComponent);
        fixture.detectChanges();
        return fixture.componentInstance;
    }

    function firstFilter(): IssuesFilterParams {
        return setFilter.mock.calls[0][0] as IssuesFilterParams;
    }

    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    it('sends one filter for a burst of typing in the title', () => {
        const component = render();

        for (const value of ['l', 'lo', 'log', 'logi', 'login']) {
            component.form.patchValue({ title: value });
        }
        expect(setFilter).not.toHaveBeenCalled();

        vi.advanceTimersByTime(300);

        expect(setFilter).toHaveBeenCalledTimes(1);
        expect(firstFilter().title).toBe('login');
    });

    it('applies a state change without waiting for the debounce', () => {
        const component = render();

        component.form.patchValue({ idsState: [4] });

        expect(setFilter).toHaveBeenCalledTimes(1);
        expect(firstFilter().idsState).toEqual([4]);
    });
});

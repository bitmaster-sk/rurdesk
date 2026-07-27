import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { ProjectStateComponent } from './project-state.component';
import { StateApi } from '../../api/state.api.service';
import { StateStore } from '../../store/state.store';
import { ProjectService } from '../../../project/project.service';
import { WindowService } from '../../../shared/window/window.service';

/**
 * Behavioural guard for the p-table → CDK row-reorder migration (R9): CDK's
 * cdkDropListDropped does NOT mutate the array (unlike PrimeNG onRowReorder), so
 * the handler must reorder a copy + set the signal before reading currentIndex.
 * A regression here silently persists the wrong orderRank.
 */
describe('ProjectStateComponent reorder (browser)', () => {
    const state = (id: number, rank: number) => ({
        idState: id,
        idProject: 10,
        name: `S${id}`,
        start: false,
        final: false,
        protected: false,
        orderRank: rank
    });

    let stateApi: { load$: any; update$: any; delete$: any };

    function setup() {
        stateApi = {
            load$: vi.fn().mockReturnValue(of([state(1, 1), state(2, 2), state(3, 3)])),
            update$: vi.fn().mockReturnValue(of(null)),
            delete$: vi.fn().mockReturnValue(of(null))
        };

        TestBed.configureTestingModule({
            declarations: [ProjectStateComponent],
            imports: [ReactiveFormsModule, TranslateModule.forRoot()],
            providers: [
                { provide: StateApi, useValue: stateApi },
                { provide: StateStore, useValue: { load: vi.fn() } },
                {
                    provide: ProjectService,
                    useValue: { updateProject: vi.fn().mockReturnValue(of({})) }
                }
            ]
        });
        TestBed.overrideComponent(ProjectStateComponent, {
            set: { template: '', providers: [{ provide: WindowService, useValue: {} }] }
        });
        const fixture = TestBed.createComponent(ProjectStateComponent);
        fixture.componentRef.setInput('project', {
            idProject: 10,
            name: 'P',
            idStateDefault: null
        });
        fixture.detectChanges();
        return fixture;
    }

    it('moves the row to its new index and persists the moved row with the new rank', () => {
        const fixture = setup();
        const component = fixture.componentInstance as any;

        // drag row 0 (S1) down to index 2
        component.onReorder({ previousIndex: 0, currentIndex: 2 });

        const order = component.states().map((s: any) => s.idState);
        expect(order).toEqual([2, 3, 1]);

        // the moved row (S1) is now at index 2 → orderRank 3, and it (not some
        // stale item) is what gets persisted.
        const persisted = stateApi.update$.mock.calls[0][0];
        expect(persisted.idState).toBe(1);
        expect(persisted.orderRank).toBe(3);
    });
});

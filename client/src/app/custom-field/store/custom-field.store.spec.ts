import { Injector, runInInjectionContext } from '@angular/core';
import { Subject, of } from 'rxjs';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { CustomFieldStore } from './custom-field.store';
import { CustomFieldType } from '../constants/custom-field-type.enum';
import { CustomField } from '../model/custom-field.model';
import { CustomFieldApi } from '../api/custom-field.api.service';

function field(idCustomField: number, key: string, name = key): CustomField {
    return {
        idCustomField,
        idProject: 1,
        key,
        name,
        fieldType: CustomFieldType.Text,
        isRequired: false,
        requiredSince: null,
        defaultValue: null,
        orderRank: idCustomField,
        archivedAt: null,
        options: []
    };
}

abstract class Emitted {
    public static latest<T>(obs: { subscribe: (fn: (v: T) => void) => unknown }): T {
        let value!: T;
        obs.subscribe(x => (value = x));
        return value;
    }
}

describe('CustomFieldStore', () => {
    let notices: Subject<{ action: NoticeAction; payload: CustomField }>;

    function create(load$: () => unknown): CustomFieldStore {
        notices = new Subject();
        const injector = Injector.create({
            providers: [
                { provide: CustomFieldApi, useValue: { load$ } },
                { provide: NoticeService, useValue: { customField$: notices.asObservable() } }
            ]
        });
        return runInInjectionContext(injector, () => new CustomFieldStore());
    }

    function build(initial: CustomField[]): CustomFieldStore {
        const store = create(() => of(initial));
        store.load();
        return store;
    }

    it('loads once through ensureLoaded and not again', () => {
        const load$ = vi.fn().mockReturnValue(of([field(1, 'note')]));
        const store = create(load$);

        store.ensureLoaded();
        store.ensureLoaded();

        expect(load$).toHaveBeenCalledTimes(1);
        expect(Emitted.latest(store.customFieldsByProject$(1)).map(f => f.key)).toEqual(['note']);
    });

    it('replaces a field on an update notice', () => {
        const store = build([field(1, 'note', 'Note')]);

        store.applyNotice({ action: NoticeAction.Update, payload: field(1, 'note', 'Poznámka') });

        expect(Emitted.latest(store.customFieldsByProject$(1))[0].name).toBe('Poznámka');
    });

    it('drops a field on a delete notice', () => {
        const store = build([field(1, 'note'), field(2, 'impact')]);

        store.applyNotice({ action: NoticeAction.Delete, payload: field(1, 'note') });

        expect(Emitted.latest(store.customFieldsByProject$(1)).map(f => f.key)).toEqual(['impact']);
    });

    it('adds a field on a create notice', () => {
        const store = build([field(1, 'note')]);

        store.applyNotice({ action: NoticeAction.Create, payload: field(2, 'impact') });

        expect(Emitted.latest(store.customFieldsByProject$(1)).map(f => f.key)).toEqual([
            'note',
            'impact'
        ]);
    });

    it('takes a websocket notice without anyone wiring it up', () => {
        const store = build([field(1, 'note', 'Note')]);

        notices.next({ action: NoticeAction.Update, payload: field(1, 'note', 'Poznámka') });

        expect(Emitted.latest(store.customFieldsByProject$(1))[0].name).toBe('Poznámka');
    });
});

import { Injector, runInInjectionContext } from '@angular/core';
import { NEVER, Subject, of, throwError } from 'rxjs';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeSubject } from 'src/app/shared/notice/constant/notice-subject.enum';
import { TagApi } from '../api/tag.api.service';
import { Tag } from '../model/tag.model';
import { TagStore } from './tag.store';

const tag = (idTag: number, idProject: number, name: string, orderRank: number): Tag => ({
    idTag,
    idProject,
    name,
    color: '#16a34a',
    orderRank,
    issueCount: 0
});

abstract class Emitted {
    public static latest<T>(obs: { subscribe: (fn: (v: T) => void) => unknown }): T {
        let value!: T;
        obs.subscribe(x => (value = x));
        return value;
    }
}

describe('TagStore', () => {
    let notices: Subject<Notice<Tag>>;
    let api: {
        load$: ReturnType<typeof vi.fn>;
        insert$: ReturnType<typeof vi.fn>;
        update$: ReturnType<typeof vi.fn>;
    };

    function build(tags: Tag[]): TagStore {
        notices = new Subject<Notice<Tag>>();
        api = {
            load$: vi.fn(() => of(tags)),
            insert$: vi.fn(() => NEVER),
            update$: vi.fn(() => NEVER)
        };
        const injector = Injector.create({
            providers: [
                { provide: TagApi, useValue: api },
                { provide: NoticeService, useValue: { tag$: notices.asObservable() } }
            ]
        });
        const store = runInInjectionContext(injector, () => new TagStore());
        store.load();
        return store;
    }

    function names(store: TagStore, idProject: number): string[] {
        return Emitted.latest(store.tagsByProject$(idProject)).map(t => t.name);
    }

    it('lists the tags of one project in their order', () => {
        const store = build([tag(1, 1, 'wiki', 2), tag(2, 1, 'next', 1), tag(3, 2, 'other', 1)]);

        expect(names(store, 1)).toEqual(['next', 'wiki']);
    });

    it('shows a renamed tag under its new name', () => {
        const store = build([tag(1, 1, 'next', 1)]);
        api.load$.mockReturnValue(NEVER);

        notices.next({
            subject: NoticeSubject.Tag,
            action: NoticeAction.Update,
            payload: tag(1, 1, 'v1.6.0', 1)
        });

        expect(names(store, 1)).toEqual(['v1.6.0']);
    });

    it('drops a deleted tag and remembers it was deleted', () => {
        const store = build([tag(1, 1, 'next', 1), tag(2, 1, 'wiki', 2)]);
        api.load$.mockReturnValue(NEVER);

        notices.next({
            subject: NoticeSubject.Tag,
            action: NoticeAction.Delete,
            payload: tag(1, 1, 'next', 1)
        });

        expect(names(store, 1)).toEqual(['wiki']);
        expect(store.isDeleted(1)).toBe(true);
        expect(store.isDeleted(2)).toBe(false);
    });

    it('reloads after another user moves a tag, since the whole project was renumbered', () => {
        const store = build([tag(1, 1, 'a', 1), tag(2, 1, 'b', 2), tag(3, 1, 'c', 3)]);
        api.load$.mockReturnValue(of([tag(3, 1, 'c', 1), tag(1, 1, 'a', 2), tag(2, 1, 'b', 3)]));

        notices.next({
            subject: NoticeSubject.Tag,
            action: NoticeAction.Update,
            payload: tag(3, 1, 'c', 1)
        });

        expect(names(store, 1)).toEqual(['c', 'a', 'b']);
    });

    it('shows a dragged tag in its new place before the server answers', () => {
        const store = build([tag(1, 1, 'a', 1), tag(2, 1, 'b', 2), tag(3, 1, 'c', 3)]);
        const renumbered = [tag(3, 1, 'c', 1), tag(1, 1, 'a', 2), tag(2, 1, 'b', 3)];

        store.reorder(renumbered, renumbered[0]);

        expect(names(store, 1)).toEqual(['c', 'a', 'b']);
        expect(api.update$).toHaveBeenCalledWith(renumbered[0]);
    });

    it('puts the tags back when the move fails', () => {
        const store = build([tag(1, 1, 'a', 1), tag(2, 1, 'b', 2), tag(3, 1, 'c', 3)]);
        api.update$.mockReturnValue(throwError(() => new Error('offline')));
        const renumbered = [tag(3, 1, 'c', 1), tag(1, 1, 'a', 2), tag(2, 1, 'b', 3)];

        store.reorder(renumbered, renumbered[0]);

        expect(names(store, 1)).toEqual(['a', 'b', 'c']);
    });

    it('knows no tag ids until the list has loaded', () => {
        notices = new Subject<Notice<Tag>>();
        const injector = Injector.create({
            providers: [
                { provide: TagApi, useValue: { load$: () => NEVER } },
                { provide: NoticeService, useValue: { tag$: notices.asObservable() } }
            ]
        });
        const store = runInInjectionContext(injector, () => new TagStore());

        expect(store.knownTagIds()).toBeNull();
    });

    it('keeps a single copy when the create notice follows the local insert', () => {
        const store = build([]);
        api.insert$.mockReturnValue(of(tag(5, 1, 'next', 1)));

        store.insert$({ idProject: 1, name: 'next', color: '#16a34a' }).subscribe();
        notices.next({
            subject: NoticeSubject.Tag,
            action: NoticeAction.Create,
            payload: tag(5, 1, 'next', 1)
        });

        expect(names(store, 1)).toEqual(['next']);
    });

    it('resolves tags by id', () => {
        const store = build([tag(7, 1, 'next', 1)]);

        expect(Emitted.latest(store.tagsMap$).get(7)?.name).toBe('next');
    });
});

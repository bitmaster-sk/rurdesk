import { describe, it, expect, vi } from 'vitest';
import { PeopleCommands } from './people.commands';

const t = (k: string) => k;

describe('PeopleCommands.build', () => {
    it('one command per member in people mode (name is data)', () => {
        const pick = vi.fn();
        const cmds = PeopleCommands.build(
            { idProject: 4, issue: null },
            [{ idUser: 1, name: 'Petra', email: 'p@x' }] as any,
            pick,
            t
        );
        expect(cmds.map(c => c.title)).toEqual(['Petra']);
        expect(cmds[0].modes).toContain('people');
        cmds[0].run();
        expect(pick).toHaveBeenCalledWith(1);
    });
    it('empty without a project', () => {
        expect(
            PeopleCommands.build({ idProject: null, issue: null }, [] as any, () => {}, t)
        ).toEqual([]);
    });
});

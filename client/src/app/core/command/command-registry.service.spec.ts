import { describe, it, expect } from 'vitest';
import { of } from 'rxjs';
import { CommandRegistryService } from './command-registry.service';
import { Command, CommandContext, CommandProvider } from './command.model';

const ctx: CommandContext = { idProject: 1, issue: null };
const cmd = (id: string): Command => ({
    id,
    title: id,
    group: 'G',
    icon: 'i',
    modes: ['commands'],
    run: () => {}
});
const provider = (commands: Command[], onPrime?: () => void): CommandProvider => ({
    prime: onPrime
        ? () => {
              onPrime();
              return of(null);
          }
        : undefined,
    getCommands: () => commands
});

describe('CommandRegistryService', () => {
    it('collects from every registered provider', () => {
        const svc = new CommandRegistryService();
        svc.register(provider([cmd('a')]));
        svc.register(provider([cmd('b')]));
        expect(svc.collect(ctx).map(c => c.id)).toEqual(['a', 'b']);
    });
    it('returns empty with no providers', () => {
        expect(new CommandRegistryService().collect(ctx)).toEqual([]);
    });
    it('primes only providers that define prime', () => {
        let calls = 0;
        const svc = new CommandRegistryService();
        svc.register(provider([cmd('a')], () => (calls += 1)));
        svc.register(provider([cmd('b')]));
        svc.prime(ctx).subscribe();
        expect(calls).toBe(1);
    });
    it('unregister removes a provider', () => {
        const svc = new CommandRegistryService();
        const p = provider([cmd('a')]);
        svc.register(p);
        svc.unregister(p);
        expect(svc.collect(ctx)).toEqual([]);
    });
});

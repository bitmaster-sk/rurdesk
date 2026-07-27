import { Injectable } from '@angular/core';
import { Observable, forkJoin, of } from 'rxjs';
import { map } from 'rxjs/operators';
import { Command, CommandContext, CommandProvider } from './command.model';

@Injectable({ providedIn: 'root' })
export class CommandRegistryService {
    private readonly providers: CommandProvider[] = [];

    public register(provider: CommandProvider): void {
        if (!this.providers.includes(provider)) this.providers.push(provider);
    }

    public unregister(provider: CommandProvider): void {
        const idx = this.providers.indexOf(provider);
        if (idx >= 0) this.providers.splice(idx, 1);
    }

    public prime(ctx: CommandContext): Observable<unknown> {
        const primes = this.providers
            .map(p => p.prime?.(ctx))
            .filter((o): o is Observable<unknown> => !!o);
        return primes.length ? forkJoin(primes).pipe(map(() => null)) : of(null);
    }

    public collect(ctx: CommandContext): Command[] {
        return this.providers.flatMap(p => p.getCommands(ctx));
    }

    public createCommands(query: string, ctx: CommandContext): Command[] {
        return this.providers
            .map(p => p.createFromQuery?.(query, ctx))
            .filter((c): c is Command => !!c);
    }
}

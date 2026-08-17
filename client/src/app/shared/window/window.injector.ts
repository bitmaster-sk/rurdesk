import { Injector, InjectOptions, ProviderToken } from '@angular/core';

/** pomáha rozšíriť injektor o ďalšie služby, hlavne WindowReference a WindowConfig  */
export class WindowInjector implements Injector {
    private readonly defaultInjector: Injector;
    private readonly extended: WeakMap<ProviderToken<unknown>, unknown>;

    public constructor(
        defaultInjector: Injector,
        extended: WeakMap<ProviderToken<unknown>, unknown>
    ) {
        this.defaultInjector = defaultInjector;
        this.extended = extended;
    }

    public get<T>(token: ProviderToken<T>, notFoundValue?: T, options?: InjectOptions): T;
    public get<T>(
        token: ProviderToken<T>,
        notFoundValue: T | null,
        options?: InjectOptions
    ): T | null;
    public get<T>(
        token: ProviderToken<T>,
        notFoundValue?: T | null,
        options?: InjectOptions
    ): T | null {
        // The extension map is heterogeneous by construction (token -> its own value), so the
        // token/value pairing is only guaranteed by whoever filled it in WindowService.open.
        const value = this.extended.get(token) as T | undefined;

        if (value) {
            return value;
        }

        // Forwarding `notFoundValue` untouched is load-bearing: `Injector.get` defaults it to
        // THROW_IF_NOT_FOUND, so substituting `null` here would turn a missing provider from a
        // loud NG0201 into a silent null.
        return this.defaultInjector.get(token, notFoundValue as T, options);
    }
}

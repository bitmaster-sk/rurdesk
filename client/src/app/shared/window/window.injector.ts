import { Injector, Type, InjectionToken, InjectOptions } from '@angular/core';

/** pomáha rozšíriť injektor o ďalšie služby, hlavne WindowReference a WindowConfig  */
export class WindowInjector implements Injector {
    constructor(
        private defaultInjector: Injector,
        private extended: WeakMap<any, any>
    ) {}

    get<T>(token: Type<T> | InjectionToken<T>, notFoundValue?: T, options?: InjectOptions): T;
    get(token: any, notFoundValue?: any);
    get(token: any, notFoundValue?: any, options?: any) {
        const value = this.extended.get(token);

        if (value) {
            return value;
        }

        return this.defaultInjector.get<any>(token, notFoundValue, options);
    }
}

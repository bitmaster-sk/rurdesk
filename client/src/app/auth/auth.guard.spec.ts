import { Injector, runInInjectionContext } from '@angular/core';
import { Router } from '@angular/router';
import { AuthGuard } from './auth.guard';
import { UserService } from './user.service';

describe('AuthGuard', () => {
    function build(hasAuth: boolean) {
        const navigate = vi.fn();
        const router = { navigate } as unknown as Router;
        const sUser = { hasAuthLocal: () => hasAuth } as unknown as UserService;
        const injector = Injector.create({
            providers: [
                { provide: Router, useValue: router },
                { provide: UserService, useValue: sUser }
            ]
        });
        const guard = runInInjectionContext(injector, () => new AuthGuard());
        return { guard, navigate };
    }

    it('allows activation when a local token exists', () => {
        const { guard, navigate } = build(true);
        expect(guard.canActivate()).toBe(true);
        expect(navigate).not.toHaveBeenCalled();
    });

    it('blocks activation and redirects to /login when no token', () => {
        const { guard, navigate } = build(false);
        expect(guard.canActivate()).toBe(false);
        expect(navigate).toHaveBeenCalledWith(['/login']);
    });
});

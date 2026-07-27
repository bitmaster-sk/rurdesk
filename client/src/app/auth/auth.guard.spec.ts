import type { Router } from '@angular/router';
import { AuthGuard } from './auth.guard';
import { UserService } from './user.service';

describe('AuthGuard', () => {
    function build(hasAuth: boolean) {
        const navigate = vi.fn();
        const router = { navigate } as unknown as Router;
        const sUser = { hasAuthLocal: () => hasAuth } as unknown as UserService;
        return { guard: new AuthGuard(router, sUser), navigate };
    }

    const route = {} as never;
    const state = {} as never;

    it('allows activation when a local token exists', () => {
        const { guard, navigate } = build(true);
        expect(guard.canActivate(route, state)).toBe(true);
        expect(navigate).not.toHaveBeenCalled();
    });

    it('blocks activation and redirects to /login when no token', () => {
        const { guard, navigate } = build(false);
        expect(guard.canActivate(route, state)).toBe(false);
        expect(navigate).toHaveBeenCalledWith(['/login']);
    });
});

import { NgModule } from '@angular/core';
import { Routes, RouterModule } from '@angular/router';
import { AuthGuard } from './auth/auth.guard';
import { AuthPage } from './auth/auth/auth.page';
import { NotFoundPage } from './home/not-found/not-found.page';
import { UserResolver } from './core/user.resolver';
import { TranslateResolver } from './translate.resolver';
import { environment } from '../environments/environment';

const routes: Routes = [
    // Dev-only design-system gallery. In production `environment.production` is a
    // static `true`, so this branch (and its lazy import) is dead-code eliminated
    // → the ui-gallery chunk is never emitted in the prod build.
    ...(environment.production
        ? []
        : [
              {
                  path: 'ui',
                  loadChildren: () =>
                      import('./ui/gallery/ui-gallery.module').then(m => m.UiGalleryModule)
              }
          ]),
    { path: 'login', component: AuthPage },
    { path: 'register', component: AuthPage },
    { path: 'logout', redirectTo: '/login' },
    // Public, auth-independent 404 so it renders whether or not you're signed in.
    // The authenticated shell's wildcard redirects unknown in-app routes here.
    { path: '404', component: NotFoundPage },
    {
        path: '',
        canActivate: [AuthGuard],
        resolve: { i18n: TranslateResolver, user: UserResolver },
        loadChildren: () => import('./home/home.module').then(m => m.HomeModule)
    }
];

@NgModule({
    imports: [RouterModule.forRoot(routes, { paramsInheritanceStrategy: 'always' })],
    exports: [RouterModule]
})
export class AppRoutingModule {}

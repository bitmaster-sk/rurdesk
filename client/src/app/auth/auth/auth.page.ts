import { Component, inject } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';

@Component({
    selector: 'app-auth',
    templateUrl: './auth.page.html',
    styleUrls: ['./auth.page.scss'],
    standalone: false
})
export class AuthPage {
    private route = inject(ActivatedRoute);

    public isLogin$ = this.isLogin();

    public isLogin(): Observable<boolean> {
        return this.route.url.pipe(map(segment => segment[0].path.includes('login')));
    }
}
